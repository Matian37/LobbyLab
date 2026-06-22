package main

import (
	"context"
	"log"
	"os"
	"server-manager/internal"
	"testing"
	"time"
)

func loadEnv() {
	os.Setenv("DBHOST", "localhost")
	os.Setenv("DBPORT", "5432")
	os.Setenv("DBUSER", "postgres")
	os.Setenv("DBPASSWORD", "123")
	os.Setenv("DBNAME", "postgres")
}

func beforeEach() {
	loadEnv()

	d, _ := NewDB(context.Background(), internal.GetMockMatchmaker())

	tables := []string{"waiting", "users", "matches"}
	for _, table := range tables {
		_, err := d.Db.ExecContext(context.Background(), "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func TestGetList(t *testing.T) {
	beforeEach()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d, err := NewDB(ctx, internal.GetMockMatchmaker())
	if err != nil {
		t.Errorf("something wrong when creating db")
	}
	err, users := d.GetList(ctx)
	if err != nil {
		t.Errorf("error while getting the list - %s", err.Error())
	}
	if len(users) != 0 {
		t.Errorf("weird thing returned by getlist - %s", users)
	}

	rows, err := d.Db.QueryContext(ctx, "INSERT INTO waiting (login) VALUES ('user1')")
	if err != nil {
		t.Errorf("couldnt make custom sql query")
	}
	defer rows.Close()

	err, users = d.GetList(ctx)
	if err != nil {
		t.Errorf("error while getting the list - %s", err.Error())
	}
	if len(users) != 1 || users[0].Login != "user1" {
		t.Errorf("weird thing returned by getlist - %s", users)
	}
}

func TestAddMatch(t *testing.T) {
	beforeEach()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := []internal.User{*internal.NewUser("user1"), *internal.NewUser("user2")}

	d, err := NewDB(ctx, internal.GetMockMatchmaker())
	if err != nil {
		t.Errorf("something wrong when creating db")
	}

	rows, err := d.Db.QueryContext(ctx, "INSERT INTO users (login, password) VALUES ('user1', 'passvvord')")
	if err != nil {
		t.Errorf("couldnt make custom sql query")
	}
	defer rows.Close()

	err = d.AddMatch(ctx, users, *internal.NewSocket("someGameServerHost", "1234"))
	if err != nil {
		t.Errorf("%v", err)
	}

	var match_id int
	var host string
	err = d.Db.QueryRowContext(ctx, "SELECT id, host FROM matches").Scan(&match_id, &host)
	if err != nil {
		t.Errorf("couldnt make custom sql query")
	}
	if host != "someGameServerHost" {
		t.Errorf("adding or reading from DB didnt work")
	}
	var match_id_user int
	err = d.Db.QueryRowContext(ctx, "SELECT match_id FROM users").Scan(&match_id_user)
	if err != nil {
		t.Errorf("couldnt make custom sql query")
	}
	if match_id != match_id_user {
		t.Errorf("assigning match id to users didnt work")
	}
}
