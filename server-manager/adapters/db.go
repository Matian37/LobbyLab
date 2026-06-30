package adapters

import (
	"context"
	"database/sql"
	"server-manager/internal"
	"time"

	"github.com/lib/pq"
)

type DatabaseConnection struct {
	Db         *sql.DB
	Listener   *pq.Listener
	Matchmaker internal.Matchmaker
}

func NewDatabaseConnection() *DatabaseConnection {
	return &DatabaseConnection{}
}

func (d *DatabaseConnection) Init(ctx context.Context, config *internal.EnvConfig) error {
	db, err := sql.Open("postgres", config.DatabaseURI)
	if err != nil {
		return err
	}

	err = db.PingContext(ctx)
	if err != nil {
		return err
	}

	listener := pq.NewListener(config.DatabaseURI, 10*time.Second, time.Minute, nil)

	d.Listener = listener
	d.Db = db
	return nil
}

func (d *DatabaseConnection) Close() error {
	err := d.Db.Close()
	if err != nil {
		return err
	}
	err = d.Listener.Close()
	if err != nil {
		return err
	}
	return nil
}

func (d *DatabaseConnection) StartListening(ctx context.Context) error {
	err := d.Listener.Listen("new_waiting_user")
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-d.Listener.Notify:
				ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
				defer cancelTimeout()
				users, err2 := d.GetList(ctxTimeout)
				if err2 != nil && len(users) > 1 {
					_ = d.Matchmaker.CreateMatches(ctxTimeout, users)
				}
			case <-time.After(90 * time.Second):
				_ = d.Listener.Ping()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (d *DatabaseConnection) GetList(ctx context.Context) ([]internal.User, error) {
	rows, err := d.Db.QueryContext(ctx, "SELECT * FROM waiting")
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

func (d *DatabaseConnection) AddMatch(ctx context.Context, users []internal.User, serverInfo internal.ServerInfo, matchId int) error {
	_, err := d.Db.ExecContext(ctx, "INSERT INTO matches (id, host, port) VALUES ($1, $2, $3)", matchId, serverInfo.Host, serverInfo.Port)
	if err != nil {
		return err
	}

	logins := make([]string, len(users))
	for i := 0; i < len(users); i++ {
		logins[i] = users[i].Login
	}

	_, err = d.Db.ExecContext(ctx, "UPDATE users SET match_id = $1 WHERE login = ANY($2)", matchId, pq.Array(logins))
	return err
}

func (d *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	_, err := d.Db.ExecContext(ctx, "INSERT INTO results (match_id, details) VALUES($1, $2)", matchID, details)
	if err != nil {
		return err
	}
	return nil
}

func (d *DatabaseConnection) GetNextMatchId(ctx context.Context) (int, error) {
	var id int
	err := d.Db.QueryRowContext(ctx, "SELECT nextval('matches_id_seq')").Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}
