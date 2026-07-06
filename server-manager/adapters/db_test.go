//go:build integration

package adapters

import (
	"context"
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

func restartSchema(t *testing.T, ctx context.Context) error {
	t.Helper()

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

func restartDB(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := restartSchema(t, ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to restart db: %v", err))
	}
}

func newHelperConn(t *testing.T) *pgx.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbConnString)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn
}

func newDBConnWithOpen(t *testing.T) *DatabaseConnection {
	d := NewDatabaseConnection(&internal.EnvConfig{DatabaseURI: dbConnString})
	require.NoError(t, d.Open(context.Background()))
	return d
}

func TestIntegration_DatabaseConnection_Open(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.Open(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("already open", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true}
		err := dc.Open(context.Background())
		assert.ErrorIs(t, err, ErrDBConnAlreadyOpen)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := NewDatabaseConnection(&internal.EnvConfig{DatabaseURI: dbConnString})
		require.NoError(t, d.Open(ctx))

		assert.Nil(t, d.listener)

		require.NotNil(t, d.conn)
		assert.NoError(t, d.conn.Ping(ctx))
	})
}

func TestIntegration_DatabaseConnection_StartListening(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{closed: true, listenerOpened: true}
		err := dc.StartListening(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("already started", func(t *testing.T) {
		dc := DatabaseConnection{listenerOpened: true}
		err := dc.StartListening(context.Background())
		assert.ErrorIs(t, err, ErrDBListenerAlreadyStarted)
	})

	t.Run("success", func(t *testing.T) {
		dc := newDBConnWithOpen(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, dc.StartListening(ctx))

		assert.True(t, dc.listenerOpened)

		require.NotNil(t, dc.listener)
		assert.NoError(t, dc.listener.Ping(ctx))
	})
}

func TestIntegration_DatabaseConnection_Close(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		dc := DatabaseConnection{closed: true}
		err := dc.Close()
		assert.ErrorIs(t, err, ErrDBConnAlreadyClosed)
	})

	t.Run("not opened", func(t *testing.T) {
		dc := DatabaseConnection{}
		assert.NoError(t, dc.Close())
		assert.True(t, dc.closed)
	})

	t.Run("success", func(t *testing.T) {
		tests := []struct {
			name           string
			listenerOpened bool
		}{
			{name: "listener not opened", listenerOpened: false},
			{name: "listener opened", listenerOpened: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				restartDB(t)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				d := newDBConnWithOpen(t)

				require.NoError(t, d.conn.Ping(ctx))
				require.NoError(t, d.Close())
				require.ErrorContains(t, d.conn.Ping(ctx), "closed")
			})
		}
	})
}

func TestIntegration_DatabaseConnection_ListenForQueueChange(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{closed: true}
		err := dc.ListenForQueueChange(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("listener not started", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true}
		err := dc.ListenForQueueChange(context.Background())
		assert.ErrorIs(t, err, ErrDBNotListening)
	})

	t.Run("context caneled", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)
		require.NoError(t, d.StartListening(ctx))

		done := make(chan error)
		go func() {
			err := d.ListenForQueueChange(ctx)
			done <- err
		}()

		cancel()

		select {
		case res := <-done:
			require.ErrorIs(t, res, ctx.Err())
		case <-time.After(2 * time.Second):
			t.Fatal("function didn't finish within timeout")
		}
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)
		require.NoError(t, d.StartListening(ctx))

		_, err := d.conn.Exec(ctx, "INSERT INTO waiting (login) VALUES ($1)", "")
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
	})
}

func TestIntegration_DatabaseConnection_GetList(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		_, err := dc.GetList(context.Background())
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		_, err := dc.GetList(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		tests := []struct {
			name          string
			expectedUsers []internal.User
		}{
			{name: "empty", expectedUsers: nil},
			{name: "one user", expectedUsers: []internal.User{{Login: "user1"}}},
			{name: "multiple users", expectedUsers: []internal.User{{Login: "user1"}, {Login: "user2"}}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				restartDB(t)

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				hc := newHelperConn(t)
				for _, user := range test.expectedUsers {
					_, err := hc.Exec(ctx, "INSERT INTO waiting (login) VALUES ($1)", user.Login)
					require.NoError(t, err)
				}

				d := newDBConnWithOpen(t)
				users, err := d.GetList(ctx)
				require.NoError(t, err)
				assert.Equal(t, users, test.expectedUsers)
			})
		}
	})
}

func TestIntegration_DatabaseConnection_AddMatch(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		err := dc.AddMatch(context.Background(), nil, internal.ServerInfo{}, 0)
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.AddMatch(context.Background(), nil, internal.ServerInfo{}, 0)
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		users := []internal.User{{Login: "user1"}, {Login: "user2"}}

		d := newDBConnWithOpen(t)

		_, err := d.conn.Exec(
			ctx, `
			INSERT INTO users (login, password)
			VALUES ('user1', ''),
				   ('user2', '')
			`,
		)
		require.NoError(t, err)

		err = d.AddMatch(ctx, users, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
		require.NoError(t, err)

		var matchID int
		var host, port string
		err = d.conn.QueryRow(ctx, "SELECT id, host, port FROM matches").Scan(&matchID, &host, &port)
		require.NoError(t, err)
		require.Equal(t, "someGameServerHost", host)
		require.Equal(t, "1234/udp", port)

		var userMatchID int
		err = d.conn.QueryRow(ctx, "SELECT match_id FROM users WHERE login='user1'").Scan(&userMatchID)
		require.NoError(t, err)
		require.Equal(t, matchID, userMatchID)

		rows, err := d.conn.Query(ctx, "SELECT user_id, match_id FROM user_matches ORDER BY (user_id, match_id)")
		require.NoError(t, err)

		type UserMatch struct {
			UserID  string `db:"user_id"`
			MatchID int    `db:"match_id"`
		}
		matches, err := pgx.CollectRows(rows, pgx.RowToStructByName[UserMatch])
		require.NoError(t, err)

		expectedMatches := []UserMatch{{UserID: "user1", MatchID: matchID}, {UserID: "user2", MatchID: matchID}}
		require.Equal(t, expectedMatches, matches)
	})
}

func TestIntegration_DatabaseConnection_SaveMatchResults(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		err := dc.SaveMatchResults(context.Background(), "", 0)
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.SaveMatchResults(context.Background(), "", 0)
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("no match found", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)

		require.ErrorIs(t, d.SaveMatchResults(ctx, "{}", 123), ErrDBMatchNotFound)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn := newHelperConn(t)
		_, err := conn.Exec(ctx, "INSERT INTO matches (id, host, port) VALUES ($1, $2, $3)", 1, "", "")
		require.NoError(t, err)

		d := newDBConnWithOpen(t)

		wantJSON := `{"example": {"id": 1}}`
		require.NoError(t, d.SaveMatchResults(ctx, string(wantJSON), 1))

		var json string
		err = d.conn.QueryRow(ctx, "SELECT results FROM matches WHERE id = $1", 1).Scan(&json)
		require.NoError(t, err)
		require.Equal(t, wantJSON, json)
	})
}

func TestIntegration_DatabaseConnection_GetNextMatchId(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		_, err := dc.GetNextMatchId(context.Background())
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		_, err := dc.GetNextMatchId(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)

		id, err := d.GetNextMatchId(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, id)

		id, err = d.GetNextMatchId(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, id)
	})
}
