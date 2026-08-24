//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Matian37/multiplayer-asset/server-manager/app"
	"github.com/Matian37/multiplayer-asset/server-manager/internal"

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

func setupNATS(t *testing.T, workerCount int, started *atomic.Bool) (string, chan error) {
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

	return addr, setupNATSMock(t, addr, workerCount, started)
}

func setupNATSMock(t *testing.T, natsURI string, workerCount int, started *atomic.Bool) chan error {
	t.Helper()

	nc, err := nats.Connect(natsURI)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	cli, err := client.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	errChan := make(chan error, 128)

	foreachCfg := ForEachConfig{
		client:      cli,
		errChan:     errChan,
		workerCount: workerCount,
	}

	_, err = nc.Subscribe("workers.health", func(msg *nats.Msg) {
		started.Store(true)

		forEachHealthyWorker(t, foreachCfg, func(_ string, workerID string) {
			_ = nc.Publish(msg.Reply, []byte(workerID))
		})
	})
	require.NoError(t, err)

	_, err = nc.Subscribe("workers.assign.*", func(msg *nats.Msg) {
		started.Store(true)

		// extract worker id from subject
		parts := strings.SplitN(msg.Subject, ".", 3)
		require.Equal(t, 3, len(parts))
		subjectWorkerID := parts[2]

		if !isWorkerIDValid(subjectWorkerID, workerCount) {
			errChan <- fmt.Errorf("unexpected assign worker id: %s", subjectWorkerID)
			return
		}

		forMatchingWorker(t,
			MatchingConfig{
				ForEachConfig: foreachCfg,
				workerID:      subjectWorkerID,
			},
			func() { _ = msg.Respond([]byte{}) },
		)
	})
	require.NoError(t, err)

	return errChan
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

func setupTestEnvironment(t *testing.T, workerCount int) (
	natsURI string,
	started *atomic.Bool,
	errChan chan error,
) {
	t.Helper()

	restartDB(t, context.Background())
	t.Cleanup(func() { cleanupContainers(t) })

	started = &atomic.Bool{}
	natsURI, errChan = setupNATS(t, workerCount, started)

	return
}

func checkErrChan(t *testing.T, errChan chan error) {
	t.Helper()

	for {
		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("errChan error: %v", err)
			}
		default:
			return
		}
	}
}

func waitForAppStart(t *testing.T, started *atomic.Bool, appResChan chan error) {
	t.Helper()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			t.Fatal("app did not start within expected time")
		default:
		}

		select {
		case <-ticker.C:
			if started.Load() {
				return
			}
		case err := <-appResChan:
			if err != nil {
				t.Fatalf("app failed to start: %v", err)
			}
		}
	}
}

func runApp(t *testing.T, ctx context.Context, cfg *internal.EnvConfig) chan error {
	t.Helper()

	resChan := make(chan error, 1)
	go func() {
		logger := slog.New(
			slog.NewTextHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: slog.LevelDebug},
			),
		)
		err := app.Run(ctx, cfg, logger)
		resChan <- err
	}()
	return resChan
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

	workerCount := 2
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := runApp(t, ctx, cfg)

	waitForAppStart(t, started, appResult)
	cancel()

	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	checkErrChan(t, errChan)
}

func runZombieContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	cli, err := client.New()
	require.NoError(t, err)
	defer cli.Close()

	res, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "busybox:latest",
			Cmd:   []string{"sleep", "inf"},
			Labels: map[string]string{
				"com.github.multiplayer-asset.worker": "true",
			},
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cli, err := client.New()
		if err != nil {
			return
		}
		defer cli.Close()
		_, _ = cli.ContainerRemove(cleanupCtx, res.ID, client.ContainerRemoveOptions{Force: true})
	})

	_, err = cli.ContainerStart(ctx, res.ID, client.ContainerStartOptions{})
	require.NoError(t, err)

	return res.ID
}

func TestE2E_RemoveZombieWorkers(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCount := 1
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	zombieID := runZombieContainer(t, ctx)

	appResult := runApp(t, ctx, cfg)
	waitForAppStart(t, started, appResult)

	cli, err := client.New()
	require.NoError(t, err)
	defer cli.Close()

	_, err = cli.ContainerInspect(context.Background(), zombieID, client.ContainerInspectOptions{})
	require.Error(t, err, "zombie container be removed")
	require.ErrorContains(t, err, "No such container", "zombie container not removed")

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	checkErrChan(t, errChan)
}

func TestE2E_AppLifecycle(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCount := 1
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := runApp(t, ctx, cfg)

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started, appResult)

	_, err = dbConn.Exec(
		ctx, `
			INSERT INTO users (login, password, queued_until)
			VALUES ('user1', '', NOW() + INTERVAL '5 hours'), ('user2', '', NOW() + INTERVAL '5 hours')
		`,
	)
	require.NoError(t, err)

	var matchID int
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT id FROM matches LIMIT 1").Scan(&matchID)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "should matchmake users and create match")

	var waitingCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE queued_until > NOW() AND match_id IS NULL").Scan(&waitingCount)
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

	var results *string
	require.Eventually(t, func() bool {
		err := dbConn.QueryRow(ctx, "SELECT results FROM matches WHERE id = $1", matchID).Scan(&results)
		return err == nil && results != nil && len(*results) != 0
	}, 10*time.Second, 100*time.Millisecond, "should save match results to database")

	assert.JSONEq(t, `{"winner": "user1"}`, *results)

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	checkErrChan(t, errChan)
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

	workerCount := 2
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := runApp(t, ctx, cfg)

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started, appResult)

	_, err = dbConn.Exec(
		ctx,
		`INSERT INTO users (login, password, queued_until)
		VALUES
		($1,$2,NOW() + INTERVAL '5 hours'),
		($3,$4,NOW() + INTERVAL '5 hours'),
		($5,$6,NOW() + INTERVAL '5 hours'),
		($7,$8,NOW() + INTERVAL '5 hours')`,
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
		"INSERT INTO users (login, password, queued_until) VALUES ($1,$2,NOW() + INTERVAL '5 hours'),($3,$4,NOW() + INTERVAL '5 hours')",
		"user5", "pass",
		"user6", "pass",
	)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	var waitingCount int
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE queued_until > NOW() AND match_id IS NULL").Scan(&waitingCount)
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

	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE queued_until > NOW() AND match_id IS NULL").Scan(&waitingCount)
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

	checkErrChan(t, errChan)
}

func TestE2E_WorkerFailureAndRestart(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCount := 1
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := runApp(t, ctx, cfg)

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started, appResult)

	_, err = dbConn.Exec(
		ctx,
		"INSERT INTO users (login, password, queued_until) VALUES ($1,$2,NOW() + INTERVAL '5 hours'),($3,$4,NOW() + INTERVAL '5 hours')",
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
		"INSERT INTO users (login, password, queued_until) VALUES ($1,$2,NOW() + INTERVAL '5 hours'),($3,$4,NOW() + INTERVAL '5 hours')",
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
	err = dbConn.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE queued_until > NOW() AND match_id IS NULL").Scan(&waitingCount)
	require.NoError(t, err)
	assert.Zero(t, waitingCount, "all users should be matched")

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	checkErrChan(t, errChan)
}

func TestE2E_NoMatchWithoutEnoughPlayers(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t, goleak.IgnoreCurrent())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCount := 1
	natsURI, started, errChan := setupTestEnvironment(t, workerCount)

	cfg := &internal.EnvConfig{
		Image:                  "busybox:latest",
		Workercount:            workerCount,
		ExposePorts:            network.PortSet{network.MustParsePort("8080"): {}},
		ClientPort:             network.MustParsePort("8080"),
		BrokerURI:              natsURI,
		BrokerNetworkName:      "bridge", // prevents docker network not found errors
		PublicHost:             "127.0.0.1",
		DatabaseURI:            dbConnString,
		PlayersPerRoom:         2,
		TestMakeContainerDummy: true,
	}

	appResult := runApp(t, ctx, cfg)

	dbConn, err := pgx.Connect(ctx, dbConnString)
	require.NoError(t, err)
	defer dbConn.Close(ctx)

	waitForAppStart(t, started, appResult)

	_, err = dbConn.Exec(ctx, "INSERT INTO users (login, password, queued_until) VALUES ($1,$2,NOW() + INTERVAL '5 hours')", "user1", "pass")
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

	checkErrChan(t, errChan)
}
