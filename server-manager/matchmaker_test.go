package main

import (
	"context"
	"errors"
	"server-manager/internal"
	"testing"
)

func GetTestMatchmaker() *Matchmaker {
	m := NewMatchmaker(&internal.MockWorkerManager{}, 2)
	m.Db = internal.GetMockDB()
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
			*internal.NewUser("user1"),
			*internal.NewUser("user2"),
		}, nil},

		{"more than enough users", []internal.User{
			*internal.NewUser("user3"),
			*internal.NewUser("user4"),
			*internal.NewUser("user5"),
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
