package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"server-manager/internal"
	"time"

	"github.com/lib/pq"
)

type WaitingTrigger struct {
	username string
}

type DB struct {
	base       *sql.DB
	listener   *pq.Listener
	matchmaker *Matchmaker
}

func NewDB(ctx context.Context, _matchmaker *Matchmaker) (*DB, error) {
	host, port, user, password, name := os.Getenv("DBHOST"), os.Getenv("DBPORT"), os.Getenv("DBUSER"), os.Getenv("DBPASSWORD"), os.Getenv("DBNAME")
	info := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, name)

	_db, err := sql.Open("postgres", info)
	err = _db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	_listener := pq.NewListener(info, 10*time.Second, time.Minute, nil)

	return &DB{
		matchmaker: _matchmaker,
		listener:   _listener,
		base:       _db,
	}, nil
}

func (d *DB) StartListening() error {
	err := d.listener.Listen("new_waiting_user")
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case n := <-d.listener.Notify:
				var data WaitingTrigger
				err = json.Unmarshal([]byte(n.Extra), &data)
				if err != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					err2, users := d.GetList(ctx)
					if err2 != nil && len(users) > 1 {
						d.matchmaker.CreateMatches(ctx, users)
					}
				}
			case <-time.After(90 * time.Second):
				d.listener.Ping()
			}
		}
	}()

	return nil
}

func (d *DB) GetList(ctx context.Context) (error, []internal.User) {
	rows, err := d.base.QueryContext(ctx, "SELECT * FROM waiting")
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
	_, err := d.base.ExecContext(ctx, "INSERT INTO matches (host, port) VALUES ($1, $2)", socket.Host, socket.Port)
	if err != nil {
		return err
	}

	var id int
	d.base.QueryRowContext(ctx, "SELECT id FROM matches WHERE host = $1 AND port = $2", socket.Host, socket.Port).Scan(&id)

	for i := 0; i < len(users); i++ {
		d.base.ExecContext(ctx, "UPDATE users SET match_id = $1 WHERE login = $2", id, users[i].Login)
	}

	return nil
}
