package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"server-manager/internal"
	"time"

	"github.com/lib/pq"
)

type DB struct {
	Db         *sql.DB
	Listener   *pq.Listener
	Matchmaker *Matchmaker
}

func NewDB(ctx context.Context, matchmaker *Matchmaker) (*DB, error) {
	host, port, user, password, name := os.Getenv("DBHOST"), os.Getenv("DBPORT"), os.Getenv("DBUSER"), os.Getenv("DBPASSWORD"), os.Getenv("DBNAME")
	info := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, name)

	db, err := sql.Open("postgres", info)
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	listener := pq.NewListener(info, 10*time.Second, time.Minute, nil)

	return &DB{
		Matchmaker: matchmaker,
		Listener:   listener,
		Db:         db,
	}, nil
}

func (d *DB) StartListening(ctx context.Context) error {
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
				err2, users := d.GetList(ctxTimeout)
				if err2 != nil && len(users) > 1 {
					d.Matchmaker.CreateMatches(ctxTimeout, users)
				}
			case <-time.After(90 * time.Second):
				d.Listener.Ping()
			}
		}
	}()

	return nil
}

func (d *DB) GetList(ctx context.Context) (error, []internal.User) {
	rows, err := d.Db.QueryContext(ctx, "SELECT * FROM waiting")
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var users []internal.User
	for rows.Next() {
		var u internal.User
		rows.Scan(&u.Login)
		users = append(users, u)
	}
	return nil, users
}

func (d *DB) AddMatch(ctx context.Context, users []internal.User, socket internal.Socket) error {
	var id int
	err := d.Db.QueryRowContext(ctx, "INSERT INTO matches (host, port) VALUES ($1, $2) RETURNING id", socket.Host, socket.Port).Scan(&id)
	if err != nil {
		return err
	}

	logins := make([]string, len(users))
	for i := 0; i < len(users); i++ {
		logins[i] = users[i].Login
	}

	_, err = d.Db.ExecContext(ctx, "UPDATE users SET match_id = $1 WHERE login = ANY($2)", id, pq.Array(logins))
	return err
}
