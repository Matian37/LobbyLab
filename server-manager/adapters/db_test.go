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
					_, err := hc.Exec(
						ctx,
						`
						INSERT INTO users (login, password, queued_until)
						VALUES ($1, '', NOW() + INTERVAL '5 hours')
						`,
						user.Login,
					)
					require.NoError(t, err)
				}

				d := newDBConnWithOpen(t)
				d.config.PlayersPerRoom = test.playersPerRoom

				users, err := d.GatherMatchPlayers(ctx)
				require.ErrorIs(t, err, test.wantErr)
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
			ctx,
			`
			INSERT INTO users (login, password, queued_until)
			VALUES
				('user1', '', NOW() + INTERVAL '5 hours'),
				('user2', '', NOW() + INTERVAL '5 hours')
			`,
		)
		require.NoError(t, err)

		err = d.AddMatch(ctx, matchUsers, internal.ServerInfo{Host: "someGameServerHost", Port: "1234/udp"}, 1)
		require.NoError(t, err)

		var matchID int
		var host, port string
		err = d.conn.QueryRow(
			ctx,
			"SELECT id, host, port FROM matches",
		).Scan(&matchID, &host, &port)
		require.NoError(t, err)
		require.Equal(t, "someGameServerHost", host)
		require.Equal(t, "1234/udp", port)

		var userMatchID int
		var queuedUntil *time.Time
		err = d.conn.QueryRow(
			ctx,
			`
			SELECT match_id, queued_until
			FROM users
			WHERE login = 'user1'
			`,
		).Scan(&userMatchID, &queuedUntil)
		require.NoError(t, err)
		assert.Equal(t, matchID, userMatchID)
		require.Nil(t, queuedUntil)

		rows, err := d.conn.Query(
			ctx,
			`
			SELECT user_id, match_id
			FROM user_matches
			ORDER BY (user_id, match_id)
			`,
		)
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tests := []struct {
			Name    string
			Success bool
			Details []byte
		}{
			{
				Name:    "successful match",
				Success: true,
				Details: []byte(`{"example1": {"id1": 1}}`),
			},
			{
				Name:    "canceled match",
				Success: false,
				Details: []byte(`{"example2": {"id2": 1}}`),
			},
		}
		for _, test := range tests {
			t.Run(test.Name, func(t *testing.T) {
				restartDB(t)

				conn := newHelperConn(t)
				_, err := conn.Exec(
					ctx,
					`
					INSERT INTO matches (id, host, port, active)
					VALUES
						(1, '', '', true),
						(2, '', '', true)
					`,
				)
				require.NoError(t, err)

				d := newDBConnWithOpen(t)
				require.NoError(t, d.SaveMatchResults(ctx, internal.Result{
					Success: test.Success,
					MatchID: 1,
					Details: test.Details,
				}))

				var json string
				var canceled, active bool
				err = d.conn.QueryRow(
					ctx,
					`
					SELECT results, canceled, active
					FROM matches
					WHERE id = 1
					`,
				).Scan(&json, &canceled, &active)
				require.NoError(t, err)

				assert.JSONEq(t, string(test.Details), json)
				assert.Equal(t, !test.Success, canceled)
				assert.False(t, active)
			})
		}
	})

	t.Run("rollback", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn := newHelperConn(t)
		_, err := conn.Exec(
			ctx,
			`
			INSERT INTO matches (id, host, port)
			VALUES (1, '', '')
			`,
		)
		require.NoError(t, err)

		_, err = conn.Exec(
			ctx,
			`
			INSERT INTO users (login, password, match_id)
			VALUES ('player1', '', 1)
			`,
		)
		require.NoError(t, err)

		d := newDBConnWithOpen(t)

		err = d.SaveMatchResults(ctx, internal.Result{
			Success: true,
			MatchID: 999,
			Details: []byte("{}"),
		})
		require.ErrorIs(t, err, ErrDBMatchNotFound)

		var matchID int
		err = d.conn.QueryRow(
			ctx,
			`
			SELECT match_id
			FROM users
			WHERE login = 'player1'
			`,
		).Scan(&matchID)
		require.NoError(t, err)
		assert.Equal(t, 1, matchID, "user match_id should be preserved after rollback")
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

func TestIntegration_DatabaseConnection_GenerateAuthTokens(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		_, err := dc.GenerateAuthTokens(context.Background(), nil)
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		_, err := dc.GenerateAuthTokens(context.Background(), nil)
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		d := newDBConnWithOpen(t)
		helperConn := newHelperConn(t)

		_, err := helperConn.Exec(
			ctx,
			`
			INSERT INTO users (login, password)
			VALUES
				('user1', ''),
				('user2', '')
			`,
		)
		require.NoError(t, err)

		users, err := d.GenerateAuthTokens(ctx, []internal.User{{Login: "user1"}, {Login: "user2"}})
		require.NoError(t, err)

		assert.Len(t, users, 2)
		assert.Equal(t, users[0].Login, "user1")
		assert.Equal(t, users[1].Login, "user2")
		assert.NotEmpty(t, users[0].MatchAuthToken)
		assert.NotEmpty(t, users[1].MatchAuthToken)
		assert.NotEqual(t, users[0].MatchAuthToken, users[1].MatchAuthToken)

		rows, err := helperConn.Query(
			ctx,
			`
			SELECT match_auth_token, login
			FROM users
			ORDER BY login
			`,
		)
		require.NoError(t, err)

		dbUsers, err := pgx.CollectRows(rows, pgx.RowToStructByName[internal.User])
		require.NoError(t, err)
		assert.Equal(t, users, dbUsers)
	})
}

func TestIntegration_DatabaseConnection_RemoveMatchStatus(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		err := dc.RemoveMatchStatus(context.Background(), 0)
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.RemoveMatchStatus(context.Background(), 0)
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		helperConn := newHelperConn(t)
		_, err := helperConn.Exec(
			ctx,
			`
			INSERT INTO matches (host, port, id)
			VALUES
				('', '0', 1),
				('', '0', 2)
			`,
		)
		require.NoError(t, err)

		_, err = helperConn.Exec(
			ctx,
			`
			INSERT INTO users (login, password, match_id, queued_until, match_auth_token)
			VALUES
				('user1', '', 1, NOW() + INTERVAL '5 hours', 't1'),
				('user2', '', NULL, NOW() + INTERVAL '5 hours', NULL),
				('user3', '', 2, NOW() + INTERVAL '5 hours', 't2'),
				('user4', '', 1, NOW() + INTERVAL '5 hours', 't3')
			`,
		)
		require.NoError(t, err)

		d := newDBConnWithOpen(t)
		require.NoError(t, d.RemoveMatchStatus(ctx, 1))

		rows, err := d.conn.Query(
			ctx,
			`
			SELECT
				login,
				match_id,
				queued_until IS NULL AS notQueued,
				match_auth_token AS matchAuthToken
			FROM users
			ORDER BY login
			`,
		)
		require.NoError(t, err)

		type User struct {
			Login          string
			MatchID        *int
			NotQueued      bool
			MatchAuthToken *string
		}
		users, err := pgx.CollectRows(rows, pgx.RowToStructByName[User])
		require.NoError(t, err)

		require.Equal(t, users, []User{
			{Login: "user1", MatchID: nil, NotQueued: true, MatchAuthToken: nil},
			{Login: "user2", MatchID: nil, NotQueued: false, MatchAuthToken: nil},
			{Login: "user3", MatchID: new(2), NotQueued: false, MatchAuthToken: new("t2")},
			{Login: "user4", MatchID: nil, NotQueued: true, MatchAuthToken: nil},
		})
	})
}

func TestIntegration_SetupMatchmaking(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		dc := DatabaseConnection{}
		err := dc.SetupMatchmaking(context.Background())
		assert.ErrorIs(t, err, ErrDBConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DatabaseConnection{connOpened: true, closed: true}
		err := dc.SetupMatchmaking(context.Background())
		assert.ErrorIs(t, err, ErrDBConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		restartDB(t)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		helperConn := newHelperConn(t)
		_, err := helperConn.Exec(
			ctx,
			`
			INSERT INTO matches (id, host, port, active, canceled, results)
			VALUES
				(1, '', '', false, true, '{}'),
				(2, '', '', false, false, '{"example": {"id": 1}}'),
				(3, '', '', true, false, NULL)
			`,
		)
		require.NoError(t, err)

		_, err = helperConn.Exec(
			ctx,
			`
			INSERT INTO users (login, password, match_id, queued_until, match_auth_token)
			VALUES
				('user1', '', 1, NULL, 't1'),
				('user2', '', NULL, NULL, NULL),
				('user3', '', NULL, NOW() + INTERVAL '5 hours', NULL)
			`,
		)
		require.NoError(t, err)

		d := newDBConnWithOpen(t)
		require.NoError(t, d.SetupMatchmaking(ctx))

		rows, err := d.conn.Query(
			ctx,
			`
			SELECT
				login,
				match_id,
				queued_until IS NULL AS notQueued,
				match_auth_token AS matchAuthToken
			FROM users
			ORDER BY login
			`,
		)
		require.NoError(t, err)

		type User struct {
			Login          string
			MatchID        *int
			NotQueued      bool
			MatchAuthToken *string
		}
		users, err := pgx.CollectRows(rows, pgx.RowToStructByName[User])
		require.NoError(t, err)
		require.Equal(t, users, []User{
			{Login: "user1", MatchID: nil, NotQueued: true, MatchAuthToken: nil},
			{Login: "user2", MatchID: nil, NotQueued: true, MatchAuthToken: nil},
			{Login: "user3", MatchID: nil, NotQueued: true, MatchAuthToken: nil},
		})

		rows, err = d.conn.Query(
			ctx,
			`
			SELECT id, active, canceled, results::text
			FROM matches
			ORDER BY id
			`,
		)
		require.NoError(t, err)

		type Match struct {
			ID       int
			Active   bool
			Canceled bool
			Results  *string
		}
		matches, err := pgx.CollectRows(rows, pgx.RowToStructByName[Match])
		require.NoError(t, err)
		require.Equal(t, matches, []Match{
			{ID: 1, Active: false, Canceled: true, Results: new("{}")},
			{ID: 2, Active: false, Canceled: false, Results: new(`{"example": {"id": 1}}`)},
			{ID: 3, Active: false, Canceled: true, Results: new("{}")},
		})
	})
}
