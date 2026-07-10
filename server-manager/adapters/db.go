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

	rows, err := dc.conn.Query(ctx, "SELECT login FROM waiting LIMIT $1", dc.config.PlayersPerRoom)
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
		"INSERT INTO matches (id, host, port) VALUES ($1, $2, $3)",
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
		"UPDATE users SET match_id = $1 WHERE login = ANY($2)",
		matchID,
		logins,
	)
	if err != nil {
		return err
	}

	res, err := tx.Exec(
		ctx,
		"DELETE FROM waiting WHERE login = ANY($1)",
		logins,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() != int64(len(users)) {
		return internal.ErrDBWaitingUserDisconnected
	}

	return tx.Commit(ctx)
}

func (dc *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	res, err := dc.conn.Exec(
		ctx,
		"UPDATE matches SET results = $1 WHERE id = $2",
		details,
		matchID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("%w id=%d", ErrDBMatchNotFound, matchID)
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
