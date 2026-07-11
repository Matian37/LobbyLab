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

		require.NotNil(t, d.conn)
		assert.NoError(t, d.conn.Ping(ctx))
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
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)

		require.NoError(t, d.conn.Ping(ctx))
		require.NoError(t, d.Close())
		require.ErrorContains(t, d.conn.Ping(ctx), "closed")
	})
}

func TestIntegration_DatabaseConnection_GetMatchPlayers(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		_, err := dc.GatherMatchPlayers(context.Background())
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		_, err := dc.GatherMatchPlayers(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		tests := []struct {
			name           string
			playersPerRoom int
			users          []internal.User
			wantPlayers    []internal.User
			wantErr        error
		}{
			{
				name:           "empty",
				playersPerRoom: 1,
				users:          nil,
				wantPlayers:    nil,
				wantErr:        internal.ErrDBNotEnoughPlayers,
			},
			{
				name:           "not enough users",
				playersPerRoom: 2,
				users:          []internal.User{{Login: "user1"}},
				wantPlayers:    nil,
				wantErr:        internal.ErrDBNotEnoughPlayers,
			},
			{
				name:           "exact number of users",
				playersPerRoom: 2,
				users:          []internal.User{{Login: "user1"}, {Login: "user2"}},
				wantPlayers:    []internal.User{{Login: "user1"}, {Login: "user2"}},
				wantErr:        nil,
			},
			{
				name:           "more than enough users",
				playersPerRoom: 2,
				users:          []internal.User{{Login: "user1"}, {Login: "user2"}, {Login: "user3"}, {Login: "user4"}},
				wantPlayers:    []internal.User{{Login: "user1"}, {Login: "user2"}},
				wantErr:        nil,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				restartDB(t)

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				hc := newHelperConn(t)
				for _, user := range test.users {
					_, err := hc.Exec(ctx, "INSERT INTO users (login, password) VALUES ($1, $2)", user.Login, "")
					require.NoError(t, err)

					_, err = hc.Exec(ctx, "INSERT INTO waiting (login) VALUES ($1)", user.Login)
					require.NoError(t, err)
				}

				d := newDBConnWithOpen(t)
				d.config.PlayersPerRoom = test.playersPerRoom

				users, err := d.GatherMatchPlayers(ctx)
				assert.ErrorIs(t, err, test.wantErr)
				assert.Equal(t, users, test.wantPlayers)
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

		d := newDBConnWithOpen(t)

		matchUsers := []internal.User{{Login: "user1"}, {Login: "user2"}}

		_, err := d.conn.Exec(
			ctx, `
			INSERT INTO users (login, password)
			VALUES ('user1', ''),
				   ('user2', '')
			`,
		)
		require.NoError(t, err)
		_, err = d.conn.Exec(
			ctx, `
			INSERT INTO waiting (login)
			VALUES ('user1'),
				   ('user2')
			`,
		)
		require.NoError(t, err)

		err = d.AddMatch(ctx, matchUsers, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
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

	t.Run("waiting user disconnected", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)

		matchUsers := []internal.User{{Login: "user1"}, {Login: "user2"}}

		_, err := d.conn.Exec(
			ctx, `
			INSERT INTO users (login, password)
			VALUES ('user1', ''),
				   ('user2', '')
			`,
		)
		require.NoError(t, err)
		_, err = d.conn.Exec(
			ctx, `
			INSERT INTO waiting (login)
			VALUES ('user2')
			`,
		)
		require.NoError(t, err)

		err = d.AddMatch(ctx, matchUsers, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
		require.ErrorIs(t, err, internal.ErrDBWaitingUserDisconnected)

		var matchCount int
		err = d.conn.QueryRow(ctx, "SELECT COUNT(*) FROM matches").Scan(&matchCount)
		require.NoError(t, err)
		require.Zero(t, matchCount)

		var userMatchCount int
		err = d.conn.QueryRow(ctx, "SELECT COUNT(*) FROM user_matches").Scan(&userMatchCount)
		require.NoError(t, err)
		require.Zero(t, userMatchCount)

		var noMatchAssigned bool
		err = d.conn.QueryRow(ctx, `
			SELECT NOT EXISTS (
				SELECT 1
				FROM users
				WHERE match_id IS NOT NULL
			)
		`).Scan(&noMatchAssigned)
		require.NoError(t, err)
		require.True(t, noMatchAssigned)
	})
}

func TestIntegration_DatabaseConnection_SaveMatchResults(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		err := dc.SaveMatchResults(context.Background(), internal.Result{})
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.SaveMatchResults(context.Background(), internal.Result{})
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("no match found", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)

		require.ErrorIs(t, d.SaveMatchResults(ctx, internal.Result{MatchID: 123, Details: []byte("{}")}), ErrDBMatchNotFound)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn := newHelperConn(t)
		_, err := conn.Exec(ctx, "INSERT INTO matches (id, host, port) VALUES (1, '', ''), (2, '', '')")
		require.NoError(t, err)

		tests := []struct {
			Name    string
			MatchID int
			Success bool
			Details []byte
		}{
			{
				Name:    "successful match",
				MatchID: 1,
				Success: true,
				Details: []byte(`{"example": {"id": 1}}`),
			},
			{
				Name:    "canceled match",
				MatchID: 2,
				Success: false,
				Details: []byte(`{"example": {"id": 2}}`),
			},
		}
		for _, test := range tests {
			t.Run(test.Name, func(t *testing.T) {
				d := newDBConnWithOpen(t)

				matchResult := internal.Result{
					Success: test.Success,
					MatchID: test.MatchID,
					Details: test.Details,
				}
				require.NoError(t, d.SaveMatchResults(ctx, matchResult))

				var json string
				var canceled bool
				err = d.conn.QueryRow(
					ctx,
					"SELECT results, canceled FROM matches WHERE id = $1",
					test.MatchID,
				).Scan(&json, &canceled)
				require.NoError(t, err)

				assert.JSONEq(t, string(test.Details), json)
				assert.Equal(t, !test.Success, canceled)
			})
		}
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
