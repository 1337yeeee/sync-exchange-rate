package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	syncservice "sync-exchange-rate/internal/service/sync"
)

type fakeSyncService struct {
	mu    sync.Mutex
	calls []time.Time
	ch    chan time.Time
}

func newFakeSyncService() *fakeSyncService {
	return &fakeSyncService{
		ch: make(chan time.Time, 10),
	}
}

func (s *fakeSyncService) SyncDate(_ context.Context, date time.Time) (syncservice.Result, error) {
	s.mu.Lock()
	s.calls = append(s.calls, date)
	s.mu.Unlock()
	s.ch <- date
	return syncservice.Result{}, nil
}

type fakeTimer struct {
	channel chan time.Time
	stopped bool
	mu      sync.Mutex
}

func newFakeTimer() *fakeTimer {
	return &fakeTimer{
		channel: make(chan time.Time, 1),
	}
}

func (t *fakeTimer) C() <-chan time.Time {
	return t.channel
}

func (t *fakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	return true
}

func (t *fakeTimer) IsStopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}

func TestNewSchedulerWithValidSchedule(t *testing.T) {
	t.Parallel()

	scheduler, err := New(newFakeSyncService(), "1 0 * * *")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if scheduler == nil {
		t.Fatal("New() returned nil scheduler")
	}
}

func TestNewSchedulerRejectsInvalidSchedule(t *testing.T) {
	t.Parallel()

	_, err := New(newFakeSyncService(), "bad schedule")
	if err == nil {
		t.Fatal("New() error = nil, want validation error")
	}
}

func TestSchedulerInvokesSyncServiceOnSchedule(t *testing.T) {
	t.Parallel()

	service := newFakeSyncService()
	timerOne := newFakeTimer()
	timerTwo := newFakeTimer()
	timers := []*fakeTimer{timerOne, timerTwo}
	timerIndex := 0

	scheduler, err := newScheduler(
		service,
		"*/1 * * * *",
		func() time.Time { return time.Date(2024, time.March, 1, 10, 0, 0, 0, time.UTC) },
		func(duration time.Duration) timer {
			current := timers[timerIndex]
			timerIndex++
			return current
		},
	)
	if err != nil {
		t.Fatalf("newScheduler() error = %v", err)
	}

	scheduler.Start()
	timerOne.channel <- time.Date(2024, time.March, 1, 10, 1, 0, 0, time.UTC)

	select {
	case got := <-service.ch:
		want := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("SyncDate() called with %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("expected sync service to be called")
	}

	scheduler.Stop()
}

func TestSchedulerGracefulStop(t *testing.T) {
	t.Parallel()

	service := newFakeSyncService()
	scheduledTimer := newFakeTimer()

	scheduler, err := newScheduler(
		service,
		"1 0 * * *",
		func() time.Time { return time.Date(2024, time.March, 1, 10, 0, 0, 0, time.UTC) },
		func(duration time.Duration) timer {
			return scheduledTimer
		},
	)
	if err != nil {
		t.Fatalf("newScheduler() error = %v", err)
	}

	scheduler.Start()
	scheduler.Stop()

	if !scheduledTimer.IsStopped() {
		t.Fatal("expected timer to be stopped during graceful shutdown")
	}

	select {
	case <-service.ch:
		t.Fatal("did not expect sync call after stop")
	default:
	}
}
