//go:build integration

package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"server-manager/internal"
	"testing"
	"time"

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
	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err = db.ExecContext(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}
	if _, err = db.ExecContext(ctx, "CREATE SCHEMA public"); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	if _, err = db.ExecContext(ctx, initSQL); err != nil {
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

func newDBConnWithInit(t *testing.T) *DatabaseConnection {
	d := DatabaseConnection{}
	err := d.Init(context.Background(), &internal.EnvConfig{DatabaseURI: dbConnString})
	require.NoError(t, err)
	return &d
}

func TestIntegration_DatabaseConnection_Init(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{}
	err := d.Init(ctx, &internal.EnvConfig{DatabaseURI: dbConnString})
	require.NoError(t, err)

	require.NotNil(t, d.db)
	assert.NoError(t, d.db.PingContext(ctx))

	assert.NotNil(t, d.listener)
}

func TestIntegration_DatabaseConnection_GetList(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithInit(t)

	users, err := d.GetList(ctx)
	require.NoError(t, err)
	require.Empty(t, users)

	rows, err := d.db.QueryContext(ctx, "INSERT INTO waiting (login) VALUES ('user1')")
	require.NoError(t, err)
	defer rows.Close()

	users, err = d.GetList(ctx)
	require.NoError(t, err)
	assert.Equal(t, users, []internal.User{{Login: "user1"}})
}

func TestIntegration_DatabaseConnection_AddMatch(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	users := []internal.User{{Login: "user1"}, {Login: "user2"}}

	d := newDBConnWithInit(t)

	rows, err := d.db.QueryContext(ctx, "INSERT INTO users (login, password) VALUES ('user1', 'passvvord')")
	require.NoError(t, err)
	defer rows.Close()

	err = d.AddMatch(ctx, users, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
	require.NoError(t, err)

	var matchID int
	var host, port string
	err = d.db.QueryRowContext(ctx, "SELECT id, host, port FROM matches").Scan(&matchID, &host, &port)
	require.NoError(t, err)
	require.Equal(t, "someGameServerHost", host)
	require.Equal(t, "1234/udp", port)

	var userMatchID int
	err = d.db.QueryRowContext(ctx, "SELECT match_id FROM users WHERE login='user1'").Scan(&userMatchID)
	require.NoError(t, err)
	require.Equal(t, matchID, userMatchID)
}

func TestIntegration_DatabaseConnection_SaveMatchResults(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithInit(t)

	users := []internal.User{{Login: "user1"}, {Login: "user2"}}
	matchDetails, err := json.Marshal(users)
	require.NoError(t, err)

	err = d.SaveMatchResults(ctx, string(matchDetails), 123)
	require.NoError(t, err)

	var matchId int
	err = d.db.QueryRowContext(ctx, "SELECT match_id FROM results").Scan(&matchId)
	require.NoError(t, err)
	require.Equal(t, 123, matchId)
}

func TestIntegration_DatabaseConnection_Close(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithInit(t)

	err := d.db.PingContext(ctx)
	require.NoError(t, err)

	require.NoError(t, d.Close())

	err = d.db.PingContext(ctx)
	require.ErrorContains(t, err, "database is closed")
}

func TestIntegration_DatabaseConnection_GetNextMatchId(t *testing.T) {
	restartDB()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := newDBConnWithInit(t)

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

	d := newDBConnWithInit(t)

	require.NoError(t, d.StartListening(ctx))

	_, err := d.db.QueryContext(ctx, "INSERT INTO waiting (login) VALUES ($1)", "")
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
