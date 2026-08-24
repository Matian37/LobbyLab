package app

import (
	"context"
	"errors"
	"github.com/Matian37/multiplayer-asset/server-manager/internal/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_HandleBackoff(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := gomock.NewController(t)
		b := mocks.NewMockBackOff(ctrl)
		b.EXPECT().Reset()

		start := time.Now()
		HandleBackoff(ctx, b, nil)
		require.Less(t, time.Since(start), 30*time.Millisecond)
	})

	t.Run("error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := gomock.NewController(t)
		b := mocks.NewMockBackOff(ctrl)
		b.EXPECT().NextBackOff().Return(0 * time.Second)

		start := time.Now()
		HandleBackoff(ctx, b, errors.New(""))
		require.Less(t, time.Since(start), 30*time.Millisecond)
	})
}
