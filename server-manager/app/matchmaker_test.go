package app

import (
	"context"
	"errors"
	"server-manager/internal"
	mocks "server-manager/internal/mocks"
	"testing"
)

func GetTestMatchmaker() *Matchmaker {
	m := &Matchmaker{WorkerManager: &mocks.MockWorkerManager{}, PlayersPerRoom: 2, EnvConfig: internal.EnvConfig{DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable"}}
	m.Db = &mocks.MockDB{}
	return m
}

func TestCreateMatches(t *testing.T) {
	m := GetTestMatchmaker()
	cases := []struct {
		name     string
		users    []internal.User
		expected error
	}{
		{"not enough users", []internal.User{}, errors.New("Not enough users")},

		{"exact number of users", []internal.User{
			internal.User{Login: "user1"},
			internal.User{Login: "user2"},
		}, nil},

		{"more than enough users", []internal.User{
			internal.User{Login: "user3"},
			internal.User{Login: "user4"},
			internal.User{Login: "user5"},
		}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := m.CreateMatches(context.Background(), tc.users)
			if result == nil {
				if tc.expected != nil {
					t.Errorf("returned nil, should have returned error %s", tc.expected.Error())
				}
			} else {
				if tc.expected.Error() != result.Error() {
					t.Errorf("got error %s, expected %s", result.Error(), tc.expected.Error())
				}
			}
		})
	}
}
