package adapters

import (
	"context"
	"errors"
	"fmt"
	"server-manager/internal"

	"github.com/jackc/pgx/v5"
)

var (
	ErrDBConnNotOpen            = errors.New("connection not open")
	ErrDBConnClosed             = errors.New("connection closed")
	ErrDBConnAlreadyOpen        = errors.New("connection already open")
	ErrDBConnAlreadyClosed      = errors.New("connection already closed")
	ErrDBNotListening           = errors.New("listener not started")
	ErrDBListenerAlreadyStarted = errors.New("listener already started")
	ErrDBMatchNotFound          = errors.New("match not found")
)

type DatabaseConnection struct {
	conn   *pgx.Conn
	config *internal.EnvConfig

	connOpened bool
	closed     bool
}

func NewDatabaseConnection(config *internal.EnvConfig) *DatabaseConnection {
	return &DatabaseConnection{config: config}
}

func (dc *DatabaseConnection) Open(ctx context.Context) error {
	if dc.closed {
		return ErrDBConnClosed
	}
	if dc.connOpened {
		return ErrDBConnAlreadyOpen
	}

	conn, err := pgx.Connect(ctx, dc.config.DatabaseURI)
	if err != nil {
		return err
	}
	dc.conn = conn

	if err = conn.Ping(ctx); err != nil {
		return err
	}
	dc.connOpened = true

	return nil
}

func (dc *DatabaseConnection) Close() error {
	if dc.closed {
		return ErrDBConnAlreadyClosed
	}

	if dc.conn != nil {
		_ = dc.conn.Close(context.Background())
	}
	dc.closed = true

	return nil
}

func (dc *DatabaseConnection) GatherMatchPlayers(ctx context.Context) ([]internal.User, error) {
	if !dc.connOpened {
		return nil, ErrDBConnNotOpen
	}
	if dc.closed {
		return nil, ErrDBConnClosed
	}

	rows, err := dc.conn.Query(
		ctx,
		`
		SELECT login, '' AS matchAuthToken
		FROM users
		WHERE queued_until > NOW()
			AND match_id IS NULL
		LIMIT $1
		`,
		dc.config.PlayersPerRoom,
	)
	if err != nil {
		return nil, err
	}

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[internal.User])
	if err != nil {
		return nil, err
	}

	if len(users) < dc.config.PlayersPerRoom {
		return nil, internal.ErrDBNotEnoughPlayers
	}
	return users, nil
}

func (dc *DatabaseConnection) AddMatch(
	ctx context.Context,
	users []internal.User,
	serverInfo internal.ServerInfo,
	matchID int,
) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	logins := make([]string, 0, len(users))
	for _, user := range users {
		logins = append(logins, user.Login)
	}

	tx, err := dc.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO matches (id, host, port)
		VALUES ($1, $2, $3)
		`,
		matchID,
		serverInfo.Host,
		serverInfo.Port,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO user_matches (user_id, match_id)
		SELECT unnest($1::text[]), $2
		`,
		logins,
		matchID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE users
		SET match_id = $1,
			queued_until = NULL
		WHERE login = ANY($2)
		`,
		matchID,
		logins,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (dc *DatabaseConnection) SaveMatchResults(ctx context.Context, results internal.Result) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	res, err := dc.conn.Exec(
		ctx,
		`
		UPDATE matches
		SET results = $1,
			canceled = $2,
			active = false
		WHERE id = $3
		`,
		results.Details,
		!results.Success,
		results.MatchID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("%w id=%d", ErrDBMatchNotFound, results.MatchID)
	}
	return nil
}

func (dc *DatabaseConnection) GetNextMatchId(ctx context.Context) (int, error) {
	if !dc.connOpened {
		return 0, ErrDBConnNotOpen
	}
	if dc.closed {
		return 0, ErrDBConnClosed
	}

	var id int
	err := dc.conn.QueryRow(ctx, "SELECT nextval('matches_id_seq')").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (dc *DatabaseConnection) GenerateAuthTokens(ctx context.Context, users []internal.User) ([]internal.User, error) {
	if !dc.connOpened {
		return nil, ErrDBConnNotOpen
	}
	if dc.closed {
		return nil, ErrDBConnClosed
	}

	logins := make([]string, 0, len(users))
	for _, user := range users {
		logins = append(logins, user.Login)
	}

	rows, err := dc.conn.Query(
		ctx,
		`
		UPDATE users
		SET match_auth_token = encode(gen_random_bytes(32), 'base64')
		WHERE login = ANY($1)
		RETURNING login, match_auth_token
		`,
		logins,
	)
	if err != nil {
		return nil, err
	}

	newUsers, err := pgx.CollectRows(rows, pgx.RowToStructByName[internal.User])
	if err != nil {
		return nil, err
	}
	return newUsers, nil
}

// removes match_id status for all users in the match
func (dc *DatabaseConnection) RemoveMatchStatus(ctx context.Context, matchID int) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	_, err := dc.conn.Exec(
		ctx,
		`
		UPDATE users
		SET
			match_id = NULL,
			queued_until = NULL,
			match_auth_token = NULL
		WHERE match_id = $1
		`,
		matchID,
	)
	return err
}

func (dc *DatabaseConnection) SetupMatchmaking(ctx context.Context) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	tx, err := dc.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// remove users from matches and matchmaking queue
	_, err = tx.Exec(
		ctx,
		`
		UPDATE users
		SET
			match_id = NULL,
			queued_until = NULL,
			match_auth_token = NULL
		`,
	)
	if err != nil {
		return err
	}

	// cancel all active matches
	_, err = tx.Exec(
		ctx,
		`
		UPDATE matches
		SET
			active = false,
			canceled = true,
			results = '{}'
		WHERE active = true
		`,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
