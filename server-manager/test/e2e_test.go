//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"server-manager/app"
	"server-manager/internal"

	"github.com/jackc/pgx/v5"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/goleak"
)

const dbImage = "postgres:18.4-alpine"
const projectName = "multiplayer-asset"

var (
	dbConnString string
	initSQL      string
)

func TestMain(m *testing.M) {
	res, err := os.ReadFile("./../../init.sql")
	if err != nil {
		panic(fmt.Sprintf("failed to read init.sql: %v", err))
	}
	initSQL = string(res)

	setupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	container, err := postgres.Run(setupCtx, dbImage, postgres.BasicWaitStrategies())
	if err != nil {
		panic(fmt.Sprintf("failed to create postgres test container: %v", err))
	}

	connString, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("failed to get postgres connection string: %v", err))
	}
	dbConnString = connString

	code := m.Run()

	if err := container.Terminate(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to terminate postgres test container: %v", err))
	}
	os.Exit(code)
}

func setupNATS(t *testing.T, started *atomic.Bool) string {
	t.Helper()

	opts := &server.Options{
		Port:      -1,
		Host:      "127.0.0.1",
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoSigs:    true,
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))

	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})

	addr := fmt.Sprintf("nats://127.0.0.1:%d", s.Addr().(*net.TCPAddr).Port)

	setupNATSMock(t, addr, started)

	return addr
}

func setupNATSMock(t *testing.T, natsURI string, started *atomic.Bool) {
	t.Helper()

	nc, err := nats.Connect(natsURI)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	cli, err := client.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	_, err = nc.Subscribe("workers.health", func(msg *nats.Msg) {
		started.Store(true)

		containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
		if err != nil {
			return
		}
		for _, c := range containers.Items {
			if isContainerMine(t, &c) {
				_ = nc.Publish(msg.Reply, []byte(c.ID))
			}
		}
	})
	require.NoError(t, err)

	_, err = nc.Subscribe("workers.assign.*", func(msg *nats.Msg) {
		started.Store(true)
		_ = msg.Respond([]byte{})
	})
	require.NoError(t, err)
}

func restartDB(t *testing.T, ctx context.Context) {
	t.Helper()

	conn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "DROP SCHEMA public CASCADE")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, "CREATE SCHEMA public")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, initSQL)
	require.NoError(t, err)
}

func cleanupContainers(t *testing.T) {
	t.Helper()

	cli, err := client.New()
	if err != nil {
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		return
	}

	for _, c := range containers.Items {
		if isContainerMine(t, &c) {
			_, _ = cli.ContainerRemove(context.Background(), c.ID, client.ContainerRemoveOptions{Force: true})
		}
	}
}

func setupTestEnvironment(t *testing.T) (natsURI string, started *atomic.Bool) {
	t.Helper()

	restartDB(t, context.Background())
	t.Cleanup(func() { cleanupContainers(t) })

	started = &atomic.Bool{}

	return setupNATS(t, started), started
}

func isContainerMine(t *testing.T, c *container.Summary) bool {
	t.Helper()
	return c.Labels["com.github.multiplayer-asset.worker"] == "true"
}

func waitForAppStart(t *testing.T, started *atomic.Bool) {
	t.Helper()
	require.Eventually(t, func() bool { return started.Load() }, 10*time.Second, 100*time.Millisecond)
}

// check if all specified users are assigned to the match
func requireAssigned(t *testing.T, dbConn *pgx.Conn, users []string, matchID int) {
	var assignedCount int
	err := dbConn.QueryRow(context.Background(), `
		SELECT COUNT(DISTINCT user_id)
		FROM user_matches
		WHERE user_id = ANY($1) AND match_id = $2;
	`, users, matchID).Scan(&assignedCount)
	require.NoError(t, err)
	assert.Equal(t, len(users), assignedCount, "all specified users should be assigned to the match")
}

func TestE2E_GracefulShutdown(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURI, started := setupTestEnvironment(t)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            2,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := make(chan error, 1)
	go func() {
		err := app.Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		appResult <- err
	}()

	waitForAppStart(t, started)
	cancel()

	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2E_AppLifecycle(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURI, started := setupTestEnvironment(t)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            1,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := make(chan error, 1)
	go func() {
		err := app.Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		appResult <- err
	}()

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started)

	_, err = dbConn.Exec(
		ctx,
		"INSERT INTO users (login, password, last_active) VALUES ($1, $2, NOW()), ($3, $4, NOW())",
		"user1", "pass1",
		"user2", "pass2",
	)
	require.NoError(t, err)

	var matchID int
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT id FROM matches LIMIT 1").Scan(&matchID)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "should matchmake users and create match")

	var waitingCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE last_active > NOW() - INTERVAL '5 seconds'").Scan(&waitingCount)
	require.NoError(t, err)
	assert.Zero(t, waitingCount)

	var host, port string
	err = dbConn.QueryRow(ctx, "SELECT host, port FROM matches WHERE id = $1", matchID).Scan(&host, &port)
	require.NoError(t, err)
	assert.Equal(t, cfg.PublicHost, host)
	assert.NotEmpty(t, port)

	requireAssigned(t, dbConn, []string{"user1", "user2"}, matchID)

	nc, err := nats.Connect(natsURI)
	require.NoError(t, err)
	defer nc.Close()

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	resultPayload, err := json.Marshal(internal.Result{
		Success: true,
		MatchID: matchID,
		Details: json.RawMessage(`{"winner": "user1"}`),
	})
	require.NoError(t, err)

	_, err = js.Publish(ctx, "workers.results", resultPayload)
	require.NoError(t, err)

	var results string
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT results FROM matches WHERE id = $1", matchID).Scan(&results)
		if err != nil || len(results) == 0 {
			return false
		}
		return true
	}, 10*time.Second, 100*time.Millisecond, "should save match results to database")

	assert.JSONEq(t, `{"winner": "user1"}`, results)

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// killWorkerContainer kills the first running container that belongs to us.
// Used to simulate a worker crash during tests.
func killWorkerContainer(t *testing.T, ctx context.Context) {
	t.Helper()
	cli, err := client.New()
	require.NoError(t, err)
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{})
	require.NoError(t, err)
	for _, c := range containers.Items {
		if isContainerMine(t, &c) {
			_, err = cli.ContainerKill(ctx, c.ID, client.ContainerKillOptions{})
			require.NoError(t, err)
			return
		}
	}
	t.Fatal("no container to kill")
}

func TestE2E_WorkersOverloadWithMatches(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURI, started := setupTestEnvironment(t)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            2,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := make(chan error, 1)
	go func() {
		appResult <- app.Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started)

	_, err = dbConn.Exec(
		ctx,
		"INSERT INTO users (login, password, last_active) VALUES ($1,$2,NOW()),($3,$4,NOW()),($5,$6,NOW()),($7,$8,NOW())",
		"user1", "pass",
		"user2", "pass",
		"user3", "pass",
		"user4", "pass",
	)
	require.NoError(t, err)

	var matchCount int
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM matches").Scan(&matchCount)
		return err == nil && matchCount == 2
	}, 10*time.Second, 100*time.Millisecond, "should create 2 matches")

	_, err = dbConn.Exec(ctx,
		"INSERT INTO users (login, password, last_active) VALUES ($1,$2,NOW()),($3,$4,NOW())",
		"user5", "pass",
		"user6", "pass",
	)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	var waitingCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE last_active > NOW() - INTERVAL '5 seconds'").Scan(&waitingCount)
	require.NoError(t, err)
	assert.Equal(t, 2, waitingCount, "users 5,6 should still wait when all workers occupied")

	nc, err := nats.Connect(natsURI)
	require.NoError(t, err)
	defer nc.Close()

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	resultPayload, err := json.Marshal(internal.Result{
		Success: true,
		MatchID: 1,
		Details: json.RawMessage(`{"winner":"user1"}`),
	})
	require.NoError(t, err)

	_, err = js.Publish(ctx, "workers.results", resultPayload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM matches").Scan(&matchCount)
		return err == nil && matchCount == 3
	}, 10*time.Second, 100*time.Millisecond, "should create 3rd match after result frees a worker")

	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE last_active > NOW() - INTERVAL '5 seconds'").Scan(&waitingCount)
	require.NoError(t, err)
	assert.Zero(t, waitingCount, "all users should be matched")

	requireAssigned(t, dbConn, []string{"user5", "user6"}, 3)

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2E_WorkerFailureAndRestart(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURI, started := setupTestEnvironment(t)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            1,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := make(chan error, 1)
	go func() {
		appResult <- app.Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started)

	_, err = dbConn.Exec(
		ctx,
		"INSERT INTO users (login, password, last_active) VALUES ($1,$2,NOW()),($3,$4,NOW())",
		"user1", "pass",
		"user2", "pass",
	)
	require.NoError(t, err)

	var matchID int
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT id FROM matches LIMIT 1").Scan(&matchID)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "should create first match")

	killWorkerContainer(t, ctx)

	_, err = dbConn.Exec(
		ctx,
		"INSERT INTO users (login, password, last_active) VALUES ($1,$2,NOW()),($3,$4,NOW())",
		"user3", "pass", "user4", "pass",
	)
	require.NoError(t, err)

	require.Eventually(t,
		func() bool {
			var canceled bool
			err := dbConn.QueryRow(
				ctx,
				"SELECT canceled FROM matches WHERE id = $1",
				matchID,
			).Scan(&canceled)
			return err == nil && canceled
		},
		30*time.Second,
		100*time.Millisecond,
		"should cancel match",
	)

	var secondMatchID int
	require.Eventually(t,
		func() bool {
			err := dbConn.QueryRow(ctx, "SELECT id FROM matches WHERE id != $1", matchID).Scan(&secondMatchID)
			return err == nil
		}, 30*time.Second, 500*time.Millisecond,
		"should restart worker and create second match (health check runs every 5s, maxPingRetries=3)",
	)

	var waitingCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE last_active > NOW() - INTERVAL '5 seconds'").Scan(&waitingCount)
	require.NoError(t, err)
	assert.Zero(t, waitingCount, "all users should be matched")

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2E_NoMatchWithoutEnoughPlayers(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURI, started := setupTestEnvironment(t)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            1,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := make(chan error, 1)
	go func() {
		appResult <- app.Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started)

	_, err = dbConn.Exec(ctx, "INSERT INTO users (login, password, last_active) VALUES ($1,$2, NOW())", "user1", "pass")
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	var matchCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM matches").Scan(&matchCount)
	require.NoError(t, err)
	assert.Zero(t, matchCount, "no match should be created; one player waiting")

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}
