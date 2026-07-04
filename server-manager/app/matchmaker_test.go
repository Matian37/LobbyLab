package app

import (
	"context"
	"errors"
	"server-manager/internal"
	"server-manager/internal/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newMockMatchmaker(t *testing.T) (*mocks.MockWorkerManager, *mocks.MockDatabaseConnection, *Matchmaker) {
	ctrl := gomock.NewController(t)
	wm := mocks.NewMockWorkerManager(ctrl)
	db := mocks.NewMockDatabaseConnection(ctrl)

	m := NewMatchmaker(wm, &internal.EnvConfig{PlayersPerRoom: 2})
	m.db = db

	return wm, db, m
}

func newMockMatchmakerWithStart(t *testing.T) (*mocks.MockWorkerManager, *mocks.MockDatabaseConnection, *Matchmaker) {
	wm, db, m := newMockMatchmaker(t)
	m.opened = true
	return wm, db, m
}

func TestNewMatchmaker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		config := &internal.EnvConfig{}
		m := NewMatchmaker(nil, config)

		require.NotNil(t, m)
		assert.Same(t, config, m.config)
		assert.NotNil(t, m.db)
		assert.False(t, m.opened)
		assert.False(t, m.closed)
	})
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

	t.Run("success", func(t *testing.T) {
		_, db, m := newMockMatchmaker(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		db.EXPECT().Open(ctx).Return(nil)
		db.EXPECT().StartListening(ctx).Return(nil)
		// prevent goroutine leak
		db.EXPECT().ListenForQueueChange(ctx).AnyTimes().Return(ctx.Err())

		assert.NoError(t, m.Start(ctx))

		assert.True(t, m.opened)
		assert.False(t, m.closed)
	})
}

func TestMatchmaker_createMatches(t *testing.T) {
	t.Run("no users", func(t *testing.T) {
		_, _, m := newMockMatchmakerWithStart(t)
		users := []internal.User{}
		err := m.createMatches(context.Background(), users)
		assert.NoError(t, err)
	})

	t.Run("not enough users", func(t *testing.T) {
		_, _, m := newMockMatchmakerWithStart(t)
		users := []internal.User{{Login: "1"}}
		err := m.createMatches(context.Background(), users)
		assert.NoError(t, err)
	})

	t.Run("one match with exact number of users", func(t *testing.T) {
		wm, db, m := newMockMatchmakerWithStart(t)

		users := []internal.User{{Login: "1"}, {Login: "2"}}
		matchConfig := internal.MatchConfig{
			MatchID: 1,
			Config:  []byte(`{"Players":[{"Login":"1"},{"Login":"2"}]}`),
		}

		gomock.InOrder(
			db.EXPECT().GetNextMatchId(context.Background()).Return(1, nil),
			wm.EXPECT().AssignMatch(context.Background(), matchConfig).Return(internal.ServerInfo{}, nil),
			db.EXPECT().AddMatch(context.Background(), users, internal.ServerInfo{}, 1).Return(nil),
		)

		err := m.createMatches(context.Background(), users)
		assert.NoError(t, err)
	})

	t.Run("two matches with remaining users", func(t *testing.T) {
		wm, db, m := newMockMatchmakerWithStart(t)

		users := []internal.User{{Login: "1"}, {Login: "2"}, {Login: "3"}, {Login: "4"}}

		firstMatch := internal.MatchConfig{
			MatchID: 1,
			Config:  []byte(`{"Players":[{"Login":"1"},{"Login":"2"}]}`),
		}
		secondMatch := internal.MatchConfig{
			MatchID: 2,
			Config:  []byte(`{"Players":[{"Login":"3"},{"Login":"4"}]}`),
		}

		firstServerInfo := internal.ServerInfo{Host: "host1", Port: "port1"}
		secondServerInfo := internal.ServerInfo{Host: "host2", Port: "port2"}

		gomock.InOrder(
			db.EXPECT().GetNextMatchId(context.Background()).Return(1, nil),
			wm.EXPECT().AssignMatch(context.Background(), firstMatch).Return(firstServerInfo, nil),
			db.EXPECT().AddMatch(context.Background(), users[0:2], firstServerInfo, 1).Return(nil),

			db.EXPECT().GetNextMatchId(context.Background()).Return(2, nil),
			wm.EXPECT().AssignMatch(context.Background(), secondMatch).Return(secondServerInfo, nil),
			db.EXPECT().AddMatch(context.Background(), users[2:4], secondServerInfo, 2).Return(nil),
		)

		assert.NoError(t, m.createMatches(context.Background(), users))
	})
}

func TestMatchmaker_runMatchmaking(t *testing.T) {
	t.Run("db failure", func(t *testing.T) {
		_, db, m := newMockMatchmakerWithStart(t)
		wantErr := errors.New("")
		db.EXPECT().GetList(gomock.Any()).Return(nil, wantErr)

		err := m.runMatchmaking(context.Background())
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("success", func(t *testing.T) {
		_, db, m := newMockMatchmakerWithStart(t)
		db.EXPECT().GetList(gomock.Any()).Return([]internal.User{{}}, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		require.NoError(t, m.runMatchmaking(ctx))
	})
}

func TestMatchmaker_listenLoop(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		_, db, m := newMockMatchmakerWithStart(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		db.EXPECT().ListenForQueueChange(gomock.Any()).Return(context.Canceled)

		done := make(chan struct{})
		go func() {
			m.listenLoop(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("listenLoop did not exit")
		}
	})

	t.Run("skip on matchmaker failure", func(t *testing.T) {
		_, db, m := newMockMatchmakerWithStart(t)

		ctx, cancel := context.WithCancel(context.Background())

		wantErr := errors.New("matchmaking failed")

		gomock.InOrder(
			db.EXPECT().ListenForQueueChange(gomock.Any()).Return(nil),
			db.EXPECT().GetList(gomock.Any()).Return(nil, wantErr),
			db.EXPECT().ListenForQueueChange(gomock.Any()).Do(func(context.Context) { cancel() }).Return(context.Canceled),
		)

		done := make(chan struct{})
		go func() {
			m.listenLoop(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("listenLoop did not exit")
		}
	})

	t.Run("success", func(t *testing.T) {
		wm, db, m := newMockMatchmakerWithStart(t)

		ctx, cancel := context.WithCancel(context.Background())

		users := []internal.User{{Login: "1"}, {Login: "2"}}

		gomock.InOrder(
			db.EXPECT().ListenForQueueChange(gomock.Any()).Return(nil),
			db.EXPECT().GetList(gomock.Any()).Return(users, nil),
			db.EXPECT().GetNextMatchId(gomock.Any()).Return(1, nil),
			wm.EXPECT().AssignMatch(gomock.Any(), gomock.Any()).Return(internal.ServerInfo{}, nil),
			db.EXPECT().AddMatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
			db.EXPECT().ListenForQueueChange(gomock.Any()).Do(func(context.Context) { cancel() }).Return(context.Canceled),
		)

		done := make(chan struct{})
		go func() {
			m.listenLoop(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("listenLoop did not exit")
		}
	})
}

func TestMatchmaker_Shutdown(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		m := Matchmaker{closed: true}
		require.ErrorIs(t, m.Shutdown(), ErrMatchmakerAlreadyShutdown)
	})

	t.Run("success", func(t *testing.T) {
		_, db, m := newMockMatchmaker(t)
		db.EXPECT().Close().Return(nil)

		require.NoError(t, m.Shutdown())
		assert.True(t, m.closed)
	})

	t.Run("partially opened", func(t *testing.T) {
		_, db, m := newMockMatchmaker(t)
		db.EXPECT().Open(context.Background()).Return(errors.New(""))
		db.EXPECT().Close().Return(nil)

		require.Error(t, db.Open(context.Background()))
		assert.False(t, m.closed)

		require.NoError(t, m.Shutdown())
		assert.True(t, m.closed)
	})
}
