package app

import (
	"context"
	"errors"
	"server-manager/internal"
	mocks "server-manager/internal/mocks"
	"testing"

	"go.uber.org/mock/gomock"
)

func newMockMatchmaker(t *testing.T) (*mocks.MockWorkerManager, *mocks.MockDatabaseConnection, *Matchmaker) {
	ctrl := gomock.NewController(t)

	wm := mocks.NewMockWorkerManager(ctrl)
	db := mocks.NewMockDatabaseConnection(ctrl)

	return wm, db, &Matchmaker{
		WorkerManager:  wm,
		Db:             db,
		PlayersPerRoom: 2,
		EnvConfig: internal.EnvConfig{
			DatabaseURI: "host=localhost port=5432 user=postgres password=123 dbname=postgres sslmode=disable",
		},
	}
}

func TestCreateMatches(t *testing.T) {
	cases := []struct {
		name     string
		users    []internal.User
		expected error
	}{
		{"not enough users", []internal.User{}, errors.New("not enough users")},

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
			wm, db, m := newMockMatchmaker(t)

			if len(tc.users) >= m.PlayersPerRoom {
				gomock.InOrder(
					db.EXPECT().GetNextMatchId(gomock.Any()).Return(1, nil),
					wm.EXPECT().AssignMatch(gomock.Any(), gomock.Any()).Return(internal.ServerInfo{}, nil),
					db.EXPECT().AddMatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)
			}

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
