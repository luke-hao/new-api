package service

import (
	"context"
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/model"
)

type RefundDrainState struct {
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
}

type refundWorkTracker struct {
	mu      sync.Mutex
	state   RefundDrainState
	changed chan struct{}
}

var refundWork refundWorkTracker

func (tracker *refundWorkTracker) begin() func(error) {
	tracker.mu.Lock()
	if tracker.changed == nil {
		tracker.changed = make(chan struct{})
	}
	tracker.state.Pending++
	tracker.mu.Unlock()
	var once sync.Once
	return func(err error) {
		once.Do(func() {
			tracker.mu.Lock()
			defer tracker.mu.Unlock()
			tracker.state.Pending--
			if err != nil {
				tracker.state.Failed++
			}
			close(tracker.changed)
			tracker.changed = make(chan struct{})
		})
	}
}

func (tracker *refundWorkTracker) snapshot() RefundDrainState {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return tracker.state
}

func (tracker *refundWorkTracker) wait(ctx context.Context) error {
	for {
		tracker.mu.Lock()
		if tracker.state.Failed > 0 {
			tracker.mu.Unlock()
			return errors.New("asynchronous refund failed; reconciliation required")
		}
		if tracker.state.Pending == 0 {
			tracker.mu.Unlock()
			return nil
		}
		changed := tracker.changed
		tracker.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func GetRefundDrainState() RefundDrainState { return refundWork.snapshot() }

// DrainBillingForMigration requires external admission and background-writer
// fencing. It waits for registered refunds, persists queued batch updates and
// rejects known failures. It does not declare a whole-site cutover ready.
func DrainBillingForMigration(ctx context.Context) error {
	if err := refundWork.wait(ctx); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := model.FlushBatchQuota(ctx); err != nil {
			return err
		}
		state := model.GetBatchQuotaState()
		if !state.PendingBatch && state.QueuedEntries == 0 {
			break
		}
	}
	state := GetRefundDrainState()
	if state.Pending != 0 || state.Failed != 0 {
		return errors.New("refund state changed while draining")
	}
	return nil
}
