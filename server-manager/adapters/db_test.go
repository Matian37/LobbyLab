//go:build integration

package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"server-manager/internal"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const dbImage = "postgres:18.4-alpine"

var dbConnString, initSQL string

func TestMain(m *testing.M) {
	res, err := os.ReadFile("./../../init.sql")
	if err != nil {
		panic(fmt.Sprintf("failed to read init.sql: %v", err))
	}
	initSQL = string(res)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := postgres.Run(ctx, dbImage, postgres.BasicWaitStrategies())
	if err != nil {
		panic(fmt.Sprintf("failed to create postgres test container: %v", err))
	}

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("failed to get postgres connection string: %v", err))
	}
	dbConnString = connString

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		panic(fmt.Sprintf("failed to terminate postgres test container: %v", err))
	}
	os.Exit(code)
}

func restartSchema(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, dbConnString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer conn.Close(ctx)

	if _, err = conn.Exec(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}
	if _, err = conn.Exec(ctx, "CREATE SCHEMA public"); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	if _, err = conn.Exec(ctx, initSQL); err != nil {
		return fmt.Errorf("failed to create schema from init.sql: %w", err)
	}

	return nil
}

func restartDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := restartSchema(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to restart db: %v", err))
	}
}

func newDBConnWithOpen(t *testing.T) *DatabaseConnection {
	d := DatabaseConnection{}
	err := d.Open(context.Background(), &internal.EnvConfig{DatabaseURI: dbConnString})
	require.NoError(t, err)
	return &d
}

func TestIntegration_DatabaseConnection_Open(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{}
	err := d.Open(ctx, &internal.EnvConfig{DatabaseURI: dbConnString})
	require.NoError(t, err)

	require.NotNil(t, d.pool)
	assert.NoError(t, d.pool.Ping(ctx))

	assert.NotNil(t, d.listener)
}

func TestIntegration_DatabaseConnection_GetList(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithOpen(t)

	users, err := d.GetList(ctx)
	require.NoError(t, err)
	require.Empty(t, users)

	_, err = d.pool.Exec(ctx, "INSERT INTO waiting (login) VALUES ('user1')")
	require.NoError(t, err)

	users, err = d.GetList(ctx)
	require.NoError(t, err)
	assert.Equal(t, users, []internal.User{{Login: "user1"}})
}

func TestIntegration_DatabaseConnection_AddMatch(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	users := []internal.User{{Login: "user1"}, {Login: "user2"}}

	d := newDBConnWithOpen(t)

	_, err := d.pool.Exec(ctx, "INSERT INTO users (login, password) VALUES ('user1', 'passvvord')")
	require.NoError(t, err)

	err = d.AddMatch(ctx, users, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
	require.NoError(t, err)

	var matchID int
	var host, port string
	err = d.pool.QueryRow(ctx, "SELECT id, host, port FROM matches").Scan(&matchID, &host, &port)
	require.NoError(t, err)
	require.Equal(t, "someGameServerHost", host)
	require.Equal(t, "1234/udp", port)

	var userMatchID int
	err = d.pool.QueryRow(ctx, "SELECT match_id FROM users WHERE login='user1'").Scan(&userMatchID)
	require.NoError(t, err)
	require.Equal(t, matchID, userMatchID)
}

func TestIntegration_DatabaseConnection_SaveMatchResults(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithOpen(t)

	users := []internal.User{{Login: "user1"}, {Login: "user2"}}
	matchDetails, err := json.Marshal(users)
	require.NoError(t, err)

	err = d.SaveMatchResults(ctx, string(matchDetails), 123)
	require.NoError(t, err)

	var matchId int
	err = d.pool.QueryRow(ctx, "SELECT match_id FROM results").Scan(&matchId)
	require.NoError(t, err)
	require.Equal(t, 123, matchId)
}

func TestIntegration_DatabaseConnection_Close(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithOpen(t)

	err := d.pool.Ping(ctx)
	require.NoError(t, err)

	require.NoError(t, d.Close())

	err = d.pool.Ping(ctx)
	require.ErrorContains(t, err, "closed")
}

func TestIntegration_DatabaseConnection_GetNextMatchId(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithOpen(t)

	id, err := d.GetNextMatchId(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, id)

	id, err = d.GetNextMatchId(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, id)
}

func TestIntegration_ListenForQueueChange(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithOpen(t)

	_, err := d.pool.Exec(ctx, "INSERT INTO waiting (login) VALUES ($1)", "")
	require.NoError(t, err)

	done := make(chan error)
	go func() {
		err := d.ListenForQueueChange(context.Background())
		done <- err
	}()

	select {
	case res := <-done:
		require.NoError(t, res)
	case <-time.After(2 * time.Second):
		t.Fatal("function didn't finish within timeout")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	cancel()

	go func() {
		err := d.ListenForQueueChange(ctx)
		done <- err
	}()

	select {
	case res := <-done:
		require.ErrorIs(t, res, ctx.Err())
	case <-time.After(2 * time.Second):
		t.Fatal("function didn't finish within timeout")
	}
}
