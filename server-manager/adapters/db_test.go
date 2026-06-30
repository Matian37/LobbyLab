//go:build integration

package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"server-manager/internal"
	"server-manager/internal/mocks"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var dbConnString, initSQL string

func TestMain(m *testing.M) {
	res, err := os.ReadFile("./../../init.sql")
	if err != nil {
		panic(fmt.Sprintf("failed to read init.sql: %v", err))
	}
	initSQL = string(res)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := postgres.Run(ctx, "postgres:18.4-alpine")
	if err != nil {
		panic(fmt.Sprintf("failed to create postgres test container: %v", err))
	}

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("failed to get postgres connection string: %v", err))
	}
	dbConnString = connString

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		panic(fmt.Sprintf("failed to terminate postgres test container: %v", err))
	}

	os.Exit(code)
}

func restartSchema(ctx context.Context) error {
	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DROP SCHEMA public")
	if err != nil {
		panic(fmt.Sprintf("failed to drop schema: %v", err))
	}

	_, err = db.ExecContext(ctx, initSQL)
	if err != nil {
		panic(fmt.Sprintf("failed to create schema from init.sql: %v", err))
	}

	return nil
}

func restartDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := restartSchema(ctx)
	if err != nil {
		slog.Error("failed to restart schema", "error", err)
	}
}

func TestInit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}

	d.Init(ctx, &internal.EnvConfig{DatabaseURI: dbConnString})
	rows, err := d.Db.QueryContext(ctx, "SELECT * FROM waiting")
	if err != nil {
		t.Errorf("blad przy custom query - %v", err)
	}
	defer rows.Close()
}

func TestGetList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: dbConnString}
	err := d.Init(context.Background(), config)
	if err != nil {
		t.Errorf("%v", err)
	}
	users, err := d.GetList(ctx)
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

	users, err = d.GetList(ctx)
	if err != nil {
		t.Errorf("error while getting the list - %s", err.Error())
	}
	if len(users) != 1 || users[0].Login != "user1" {
		t.Errorf("weird thing returned by getlist - %s", users)
	}
}

func TestAddMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := []internal.User{internal.User{Login: "user1"}, internal.User{Login: "user2"}}

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: dbConnString}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := []internal.User{internal.User{Login: "user1"}, internal.User{Login: "user2"}}

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	config := &internal.EnvConfig{DatabaseURI: dbConnString}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	d.Init(ctx, &internal.EnvConfig{DatabaseURI: dbConnString})
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := DatabaseConnection{Matchmaker: &mocks.MockMatchmaker{}}
	err := d.Init(ctx, &internal.EnvConfig{DatabaseURI: dbConnString})
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
