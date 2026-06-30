package adapters

import (
	"context"
	"encoding/json"
	"log"
	"server-manager/internal"
	"server-manager/internal/mocks"
	"testing"
	"time"
)

func beforeEach() {
	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"}
	err := d.Init(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}

	tables := []string{"waiting", "users", "matches", "results"}
	for _, table := range tables {
		_, err := d.Db.ExecContext(context.Background(), "TRUNCATE TABLE "+table+" RESTART IDENTITY CASCADE")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func TestInit(t *testing.T) {
	beforeEach()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}

	d.Init(ctx, &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"})
	rows, err := d.Db.QueryContext(ctx, "SELECT * FROM waiting")
	if err != nil {
		t.Errorf("blad przy custom query - %v", err)
	}
	defer rows.Close()
}

func TestGetList(t *testing.T) {
	beforeEach()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"}
	err := d.Init(context.Background(), config)
	if err != nil {
		t.Errorf("%v", err)
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
		t.Errorf("%v", err)
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
	users := []internal.User{internal.User{Login: "user1"}, internal.User{Login: "user2"}}

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"}
	err := d.Init(context.Background(), config)
	if err != nil {
		t.Errorf("%v", err)
	}

	rows, err := d.Db.QueryContext(ctx, "INSERT INTO users (login, password) VALUES ('user1', 'passvvord')")
	if err != nil {
		t.Errorf("%v", err)
	}
	defer rows.Close()

	err = d.AddMatch(ctx, users, internal.ServerInfo{Host: "someGameServerHost", Port: "1234"}, 1)
	if err != nil {
		t.Errorf("%v", err)
	}

	var matchId int
	var host string
	err = d.Db.QueryRowContext(ctx, "SELECT id, host FROM matches").Scan(&matchId, &host)
	if err != nil {
		t.Errorf("%v", err)
	}
	if host != "someGameServerHost" {
		t.Errorf("adding or reading from DB didnt work")
	}
	var matchIdUser int
	err = d.Db.QueryRowContext(ctx, "SELECT match_id FROM users").Scan(&matchIdUser)
	if err != nil {
		t.Errorf("%v", err)
	}
	if matchId != matchIdUser {
		t.Errorf("assigning match id to users didnt work")
	}
}

func TestSaveMatchResults(t *testing.T) {
	beforeEach()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := []internal.User{internal.User{Login: "user1"}, internal.User{Login: "user2"}}

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"}
	err := d.Init(context.Background(), config)
	if err != nil {
		t.Errorf("something wrong when creating db")
	}
	details, err2 := json.Marshal(users)
	if err2 != nil {
		t.Errorf("%v", err2)
	}
	err = d.SaveMatchResults(ctx, string(details), 123)
	if err != nil {
		t.Errorf("%v", err)
	}

	var matchId int
	err = d.Db.QueryRowContext(ctx, "SELECT match_id FROM results").Scan(&matchId)
	if err != nil {
		t.Errorf("couldnt make custom sql query")
	}

	if matchId != 123 {
		t.Errorf("error in saving")
	}
}

func TestClose(t *testing.T) {
	beforeEach()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	d.Init(ctx, &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"})
	rows, err := d.Db.QueryContext(ctx, "SELECT * FROM waiting")
	if err != nil {
		t.Errorf("%v", err)
	}
	defer rows.Close()

	err = d.Close()
	if err != nil {
		t.Errorf("%v", err)
	}
	rows, err = d.Db.QueryContext(ctx, "SELECT * FROM waiting")
	if err == nil {
		t.Errorf("db didnt close properly")
	}
}

func TestGetNextMatchId(t *testing.T) {
	beforeEach()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	err := d.Init(ctx, &internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"})
	if err != nil {
		t.Errorf("%v", err)
	}
	id, err := d.GetNextMatchId(ctx)
	if err != nil {
		t.Errorf("%v", err)
	}
	if id != 1 {
		t.Errorf("wrong id returned - %d", id)
	}
	id, err = d.GetNextMatchId(ctx)
	if id != 2 {
		t.Errorf("wrong id returned - %d", id)
	}
}
