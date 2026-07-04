package app

import (
	"context"
	"server-manager/internal"
	"server-manager/internal/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newMockMatchmaker(t *testing.T) (*mocks.MockWorkerManager, *mocks.MockDatabaseConnection, *Matchmaker) {
	ctrl := gomock.NewController(t)
	wm := mocks.NewMockWorkerManager(ctrl)
	db := mocks.NewMockDatabaseConnection(ctrl)
	return wm, db, &Matchmaker{
		workerManager: wm,
		db:            db,
		config:        &internal.EnvConfig{PlayersPerRoom: 2},
	}
}

func TestCreateMatches(t *testing.T) {
	cases := []struct {
		name     string
		users    []internal.User
		expected error
	}{
		{
			name:     "not enough users",
			users:    []internal.User{},
			expected: ErrNotEnoughUsers,
		},
		{
			name: "exact number of users",
			users: []internal.User{
				{Login: "user1"},
				{Login: "user2"},
			},
			expected: nil,
		},
		{
			name: "more than enough users",
			users: []internal.User{
				{Login: "user3"},
				{Login: "user4"},
				{Login: "user5"},
			},
			expected: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wm, db, m := newMockMatchmaker(t)

			if len(tc.users) >= m.config.PlayersPerRoom {
				gomock.InOrder(
					db.EXPECT().GetNextMatchId(gomock.Any()).Return(1, nil),
					wm.EXPECT().AssignMatch(gomock.Any(), gomock.Any()).Return(internal.ServerInfo{}, nil),
					db.EXPECT().AddMatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)
			}

			err := m.createMatches(context.Background(), tc.users)
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestNewMatchmaker(t *testing.T) {
	config := &internal.EnvConfig{}
	m := NewMatchmaker(nil, config)

	require.NotNil(t, m)
	assert.Same(t, config, m.config)
	assert.NotNil(t, m.db)
	assert.False(t, m.opened)
	assert.False(t, m.closed)
}

func TestMatchmaker_Start(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		m := Matchmaker{closed: true, opened: true}
		err := m.Start(context.Background())
		assert.ErrorIs(t, err, ErrMatchmakerClosed)
	})

	t.Run("already open", func(t *testing.T) {
		m := Matchmaker{opened: true}
		err := m.Start(context.Background())
		assert.ErrorIs(t, err, ErrMatchmakerAlreadyOpen)
	})
}

func TestMatchmaker_Shutdown(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		m := Matchmaker{closed: true}
		require.ErrorIs(t, m.Shutdown(), ErrMatchmakerAlreadyShutdown)
	})
}
