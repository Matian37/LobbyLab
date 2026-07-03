package adapters

import (
	"context"
	"database/sql"
	"server-manager/internal"
	"time"

	"github.com/lib/pq"
)

type DatabaseConnection struct {
	db       *sql.DB
	listener *pq.Listener
}

func NewDatabaseConnection() *DatabaseConnection {
	return &DatabaseConnection{}
}

func (dc *DatabaseConnection) Init(ctx context.Context, config *internal.EnvConfig) error {
	db, err := sql.Open("postgres", config.DatabaseURI)
	if err != nil {
		return err
	}

	if err = db.PingContext(ctx); err != nil {
		return err
	}

	dc.db = db
	dc.listener = pq.NewListener(config.DatabaseURI, 10*time.Second, time.Minute, nil)

	return nil
}

func (dc *DatabaseConnection) Close() error {
	if err := dc.db.Close(); err != nil {
		return err
	}
	if err := dc.listener.Close(); err != nil {
		return err
	}
	return nil
}

func (dc *DatabaseConnection) StartListening(ctx context.Context) error {
	return dc.listener.Listen("new_waiting_user")
}

func (dc *DatabaseConnection) ListenForQueueChange(ctx context.Context) error {
	select {
	case <-dc.listener.Notify:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (dc *DatabaseConnection) GetList(ctx context.Context) ([]internal.User, error) {
	rows, err := dc.db.QueryContext(ctx, "SELECT * FROM waiting")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
	_, err := dc.db.ExecContext(
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

	_, err = dc.db.ExecContext(
		ctx,
		"UPDATE users SET match_id = $1 WHERE login = ANY($2)",
		matchId,
		pq.Array(logins),
	)
	return err
}

func (dc *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	_, err := dc.db.ExecContext(
		ctx,
		"INSERT INTO results (match_id, details) VALUES($1, $2)",
		matchID,
		details,
	)
	return err
}

func (dc *DatabaseConnection) GetNextMatchId(ctx context.Context) (int, error) {
	var id int
	err := dc.db.QueryRowContext(ctx, "SELECT nextval('matches_id_seq')").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}
