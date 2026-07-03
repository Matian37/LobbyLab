package adapters

import (
	"context"
	"server-manager/internal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnection struct {
	pool     *pgxpool.Pool
	listener *pgx.Conn
}

func NewDatabaseConnection() *DatabaseConnection {
	return &DatabaseConnection{}
}

func (dc *DatabaseConnection) Open(ctx context.Context, config *internal.EnvConfig) error {
	pool, err := pgxpool.New(ctx, config.DatabaseURI)
	if err != nil {
		return err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return err
	}

	dc.pool = pool

	conn, err := pgx.Connect(ctx, config.DatabaseURI)
	if err != nil {
		pool.Close()
		return err
	}

	dc.listener = conn

	if _, err := conn.Exec(ctx, "LISTEN new_waiting_user"); err != nil {
		conn.Close(ctx)
		pool.Close()
		return err
	}

	return nil
}

func (dc *DatabaseConnection) Close() error {
	if dc.listener != nil {
		_ = dc.listener.Close(context.Background())
	}
	if dc.pool != nil {
		dc.pool.Close()
	}
	return nil
}

func (dc *DatabaseConnection) ListenForQueueChange(ctx context.Context) error {
	_, err := dc.listener.WaitForNotification(ctx)
	return err
}

func (dc *DatabaseConnection) GetList(ctx context.Context) ([]internal.User, error) {
	rows, err := dc.pool.Query(ctx, "SELECT * FROM waiting")
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
	_, err := dc.pool.Exec(
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
	}

	_, err = dc.pool.Exec(
		ctx,
		"UPDATE users SET match_id = $1 WHERE login = ANY($2)",
		matchId,
		logins,
	)
	return err
}

func (dc *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	_, err := dc.pool.Exec(
		ctx,
		"INSERT INTO results (match_id, details) VALUES($1, $2)",
		matchID,
		details,
	)
	return err
}

func (dc *DatabaseConnection) GetNextMatchId(ctx context.Context) (int, error) {
	var id int
	err := dc.pool.QueryRow(ctx, "SELECT nextval('matches_id_seq')").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}
