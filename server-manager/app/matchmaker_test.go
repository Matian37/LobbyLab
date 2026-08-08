package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
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

	m := NewMatchmaker(wm, &internal.EnvConfig{PlayersPerRoom: 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.db = db
	m.dbPoolTimeout = 100 * time.Microsecond

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
		m := NewMatchmaker(nil, config, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
		wm, db, m := newMockMatchmaker(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		db.EXPECT().Open(ctx).Return(nil)
		db.EXPECT().SetupMatchmaking(ctx).Return(nil)
		wm.EXPECT().WaitForFreeWorker(gomock.Any()).MaxTimes(1)

		assert.NoError(t, m.Start(ctx))

		assert.True(t, m.opened)
		assert.False(t, m.closed)
	})
}

func TestMatchmaker_createMatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wm, db, m := newMockMatchmakerWithStart(t)

		m.config.PlayersPerRoom = 1
		serverInfo := internal.ServerInfo{Host: "localhost", Port: "1234/udp"}
		users := []internal.User{{Login: "1"}}
		wantMatchConfig := internal.MatchConfig{
			MatchID: 1,
			Config:  []byte(`{"players":[{"login":"1","matchAuthToken":""}]}`),
		}

		gomock.InOrder(
			db.EXPECT().GetNextMatchId(gomock.Any()).Return(1, nil),
			wm.EXPECT().AssignMatch(gomock.Any(), wantMatchConfig).Return(serverInfo, nil),
			db.EXPECT().AddMatch(gomock.Any(), users, serverInfo, 1).Return(nil),
		)

		matchID, err := m.createMatch(context.Background(), users)
		assert.NoError(t, err)
		assert.Equal(t, 1, matchID)
	})
}

func TestMatchmaker_waitForEnoughPlayers(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, m := newMockMatchmakerWithStart(t)
		// ensures ctx.Err() case will be done first instead of ticker
		m.dbPoolTimeout = 1 * time.Hour

		_, err := m.waitForEnoughPlayers(ctx)
		require.ErrorIs(t, err, ctx.Err())
	})

	t.Run("not enough players", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, db, m := newMockMatchmakerWithStart(t)

		gomock.InOrder(
			db.EXPECT().GatherMatchPlayers(ctx).Return(nil, internal.ErrDBNotEnoughPlayers),
			db.EXPECT().GatherMatchPlayers(ctx).Return(nil, ctx.Err()),
		)

		done := make(chan error)
		go func() {
			_, err := m.waitForEnoughPlayers(ctx)
			done <- err
		}()

		select {
		case err := <-done:
			require.ErrorIs(t, err, ctx.Err())
		case <-time.After(1 * time.Second):
			t.Fatal("waitForEnoughPlayers did not finish")
		}
	})

	t.Run("db failure", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, db, m := newMockMatchmakerWithStart(t)

		wantErr := errors.New("")
		db.EXPECT().GatherMatchPlayers(ctx).Return(nil, wantErr)

		done := make(chan error)
		go func() {
			_, err := m.waitForEnoughPlayers(ctx)
			done <- err
		}()

		select {
		case err := <-done:
			assert.ErrorIs(t, err, wantErr)
		case <-time.After(1 * time.Second):
			t.Fatal("waitForEnoughPlayers did not finish")
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, db, m := newMockMatchmakerWithStart(t)

		wantUsers := []internal.User{{Login: "1"}, {Login: "2"}}

		gomock.InOrder(
			db.EXPECT().GatherMatchPlayers(ctx).Return(nil, internal.ErrDBNotEnoughPlayers),
			db.EXPECT().GatherMatchPlayers(ctx).Return(nil, internal.ErrDBNotEnoughPlayers),
			db.EXPECT().GatherMatchPlayers(ctx).Return(wantUsers, nil),
		)

		type response struct {
			users []internal.User
			err   error
		}
		done := make(chan response)
		go func() {
			users, err := m.waitForEnoughPlayers(ctx)
			done <- response{users: users, err: err}
		}()

		select {
		case resp := <-done:
			assert.NoError(t, resp.err)
			assert.Equal(t, wantUsers, resp.users)
		case <-time.After(1 * time.Second):
			t.Fatal("waitForEnoughPlayers did not finish")
		}
	})
}

func TestMatchmaker_matchmakingLoop(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wm, _, m := newMockMatchmakerWithStart(t)

		wm.EXPECT().WaitForFreeWorker(ctx).Do(func(ctx context.Context) { cancel() })

		done := make(chan error)
		go func() {
			err := m.matchmakingLoop(ctx)
			done <- err
		}()

		select {
		case err := <-done:
			require.Error(t, err)
			assert.Equal(t, context.Canceled, err)
		case <-time.After(1 * time.Second):
			t.Fatal("matchmakingLoop did not finish")
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wm, db, m := newMockMatchmakerWithStart(t)
		m.config.PlayersPerRoom = 2

		matchUsers := []internal.User{{Login: "1"}, {Login: "2"}}
		matchConfig := internal.MatchConfig{
			Config:  []byte(`{"players":[{"login":"1","matchAuthToken":""},{"login":"2","matchAuthToken":""}]}`),
			MatchID: 1,
		}

		gomock.InOrder(
			wm.EXPECT().WaitForFreeWorker(ctx),
			db.EXPECT().GatherMatchPlayers(ctx).Return(nil, internal.ErrDBNotEnoughPlayers),
			db.EXPECT().GatherMatchPlayers(ctx).Return(matchUsers, nil),
			db.EXPECT().GenerateAuthTokens(ctx, matchUsers).Return(matchUsers, nil),
			db.EXPECT().GetNextMatchId(gomock.Any()).Return(matchConfig.MatchID, nil),
			wm.EXPECT().AssignMatch(gomock.Any(), matchConfig).Return(internal.ServerInfo{}, nil),
			db.EXPECT().AddMatch(gomock.Any(), matchUsers, internal.ServerInfo{}, matchConfig.MatchID).Return(nil),
			wm.EXPECT().WaitForFreeWorker(ctx).Do(func(ctx context.Context) { cancel() }),
		)

		done := make(chan error)
		go func() {
			err := m.matchmakingLoop(ctx)
			done <- err
		}()

		select {
		case err := <-done:
			require.Error(t, err)
			assert.Equal(t, context.Canceled, err)
		case <-time.After(1 * time.Second):
			t.Fatal("matchmakingLoop did not finish")
		}
	})
}

func TestMatchmaker_Shutdown(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		_, _, m := newMockMatchmaker(t)
		m.closed = true
		assert.NotPanics(t, m.Shutdown)
		assert.True(t, m.closed)
	})

	t.Run("success", func(t *testing.T) {
		_, db, m := newMockMatchmaker(t)
		db.EXPECT().Close().Return(nil)
		m.Shutdown()
		assert.True(t, m.closed)
	})
}
