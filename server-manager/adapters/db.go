package adapters

import (
	"context"
	"errors"
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
)

type DatabaseConnection struct {
	conn     *pgx.Conn
	listener *pgx.Conn
	config   *internal.EnvConfig

	connOpened     bool
	listenerOpened bool
	closed         bool
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

	if dc.listener != nil {
		_ = dc.listener.Close(context.Background())
	}
	if dc.conn != nil {
		_ = dc.conn.Close(context.Background())
	}
	dc.closed = true

	return nil
}

func (dc *DatabaseConnection) StartListening(ctx context.Context) error {
	if dc.closed {
		return ErrDBConnClosed
	}
	if dc.listenerOpened {
		return ErrDBListenerAlreadyStarted
	}

	listener, err := pgx.Connect(ctx, dc.config.DatabaseURI)
	if err != nil {
		return err
	}
	dc.listener = listener

	_, err = dc.listener.Exec(ctx, "LISTEN new_waiting_user")
	if err != nil {
		return err
	}

	dc.listenerOpened = true
	return nil
}

func (dc *DatabaseConnection) ListenForQueueChange(ctx context.Context) error {
	if dc.closed {
		return ErrDBConnClosed
	}
	if !dc.listenerOpened {
		return ErrDBNotListening
	}

	_, err := dc.listener.WaitForNotification(ctx)
	return err
}

func (dc *DatabaseConnection) GetList(ctx context.Context) ([]internal.User, error) {
	if !dc.connOpened {
		return nil, ErrDBConnNotOpen
	}
	if dc.closed {
		return nil, ErrDBConnClosed
	}

	rows, err := dc.conn.Query(ctx, "SELECT * FROM waiting")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []internal.User
	for rows.Next() {
		var u internal.User
		if err := rows.Scan(&u.Login); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (dc *DatabaseConnection) AddMatch(
	ctx context.Context,
	users []internal.User,
	serverInfo internal.ServerInfo,
	matchId int,
) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	_, err := dc.conn.Exec(
		ctx,
		"INSERT INTO matches (id, host, port) VALUES ($1, $2, $3)",
		matchId,
		serverInfo.Host,
		serverInfo.Port,
	)
	if err != nil {
		return err
	}

	logins := make([]string, 0, len(users))
	for _, user := range users {
		logins = append(logins, user.Login)
		_, err = dc.conn.Exec(
			ctx,
			"INSERT INTO user_matches (user_id, match_id) VALUES ($1,  $2)",
			user.Login,
			matchId,
		)
		if err != nil {
			return err
		}
	}

	_, err = dc.conn.Exec(
		ctx,
		"UPDATE users SET match_id = $1 WHERE login = ANY($2)",
		matchId,
		logins,
	)
	return err
}

func (dc *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	if !dc.connOpened {
		return ErrDBConnNotOpen
	}
	if dc.closed {
		return ErrDBConnClosed
	}

	_, err := dc.conn.Exec(
		ctx,
		"INSERT INTO results (match_id, details) VALUES($1, $2)",
		matchID,
		details,
	)
	return err
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
