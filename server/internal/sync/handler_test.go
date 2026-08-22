package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/sync/collector"
)

type fakePullChangeStoreForShutdown struct {
	shutdownCalled bool
	shutdownErr    error
}

func (s *fakePullChangeStoreForShutdown) GetChangesSinceUSN(ctx context.Context, info PullInfo, c *collector.PullCollector) (PullChangesResult, error) {
	panic("GetChangesSinceUSN should not be called")
}

func (s *fakePullChangeStoreForShutdown) GracefulShutdown() error {
	s.shutdownCalled = true
	return s.shutdownErr
}

func TestSyncHandlerGracefulShutdown(t *testing.T) {
	store := &fakePullChangeStoreForShutdown{}
	handler := &SyncHandler{pulStore: store}

	err := handler.GracefulShutdown()

	require.NoError(t, err)
	require.True(t, store.shutdownCalled)
}

func TestSyncHandlerGracefulShutdownReturnsStoreError(t *testing.T) {
	wantErr := errors.New("shutdown failed")
	store := &fakePullChangeStoreForShutdown{shutdownErr: wantErr}
	handler := &SyncHandler{pulStore: store}

	err := handler.GracefulShutdown()

	require.ErrorIs(t, err, wantErr)
	require.True(t, store.shutdownCalled)
}
