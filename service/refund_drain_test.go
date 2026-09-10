package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefundDrainTracksQueuedWorkBeforeItStarts(t *testing.T) {
	var tracker refundWorkTracker
	done := tracker.begin()
	require.Equal(t, 1, tracker.snapshot().Pending)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, tracker.wait(ctx), context.DeadlineExceeded)
	done(nil)
	done(nil)
	require.Equal(t, RefundDrainState{}, tracker.snapshot())
	require.NoError(t, tracker.wait(context.Background()))
}

func TestRefundDrainRemembersFailureAfterTaskCompletes(t *testing.T) {
	var tracker refundWorkTracker
	finish := tracker.begin()
	finish(errors.New("injected write error"))
	require.Equal(t, RefundDrainState{Failed: 1}, tracker.snapshot())
	require.Error(t, tracker.wait(context.Background()))
}

func TestRefundDrainWaitsForAllNestedWork(t *testing.T) {
	var tracker refundWorkTracker
	parent := tracker.begin()
	child := tracker.begin()
	parent(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, tracker.wait(ctx), context.DeadlineExceeded)
	child(nil)
	require.NoError(t, tracker.wait(context.Background()))
}
