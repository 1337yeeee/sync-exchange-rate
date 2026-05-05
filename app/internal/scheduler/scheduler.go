package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	syncservice "sync-exchange-rate/internal/service/sync"
)

type syncService interface {
	SyncDate(ctx context.Context, date time.Time) (syncservice.Result, error)
}

type Scheduler struct {
	service      syncService
	schedule     cronSchedule
	now          func() time.Time
	timerFactory func(time.Duration) timer

	stopCh   chan struct{}
	doneCh   chan struct{}
	startMux sync.Mutex
	started  bool
	stopOnce sync.Once
}

type timer interface {
	C() <-chan time.Time
	Stop() bool
}

type realTimer struct {
	timer *time.Timer
}

func (t *realTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *realTimer) Stop() bool {
	return t.timer.Stop()
}

type cronSchedule struct {
	minute cronField
	hour   cronField
}

type cronField struct {
	kind  cronFieldKind
	value int
}

type cronFieldKind int

const (
	cronFieldAny cronFieldKind = iota
	cronFieldExact
	cronFieldStep
)

func New(service syncService, expression string) (*Scheduler, error) {
	return newScheduler(service, expression, time.Now, func(duration time.Duration) timer {
		return &realTimer{timer: time.NewTimer(duration)}
	})
}

func newScheduler(service syncService, expression string, now func() time.Time, timerFactory func(time.Duration) timer) (*Scheduler, error) {
	if service == nil {
		return nil, fmt.Errorf("sync service must not be nil")
	}

	schedule, err := parseCronSchedule(expression)
	if err != nil {
		return nil, err
	}

	if now == nil {
		now = time.Now
	}

	if timerFactory == nil {
		timerFactory = func(duration time.Duration) timer {
			return &realTimer{timer: time.NewTimer(duration)}
		}
	}

	return &Scheduler{
		service:      service,
		schedule:     schedule,
		now:          now,
		timerFactory: timerFactory,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}, nil
}

func (s *Scheduler) Start() {
	s.startMux.Lock()
	defer s.startMux.Unlock()

	if s.started {
		return
	}

	s.started = true
	go s.run()
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})

	<-s.doneCh
}

func (s *Scheduler) run() {
	defer close(s.doneCh)

	for {
		now := s.now()
		nextRun := s.schedule.Next(now)
		waitDuration := time.Until(nextRun)
		if waitDuration < 0 {
			waitDuration = 0
		}

		scheduledTimer := s.timerFactory(waitDuration)

		select {
		case firedAt := <-scheduledTimer.C():
			syncDate := toSyncDate(firedAt)
			result, err := s.service.SyncDate(context.Background(), syncDate)
			if err != nil {
				log.Printf("scheduled sync failed: date=%s error=%v", syncDate.Format("2006-01-02"), err)
				continue
			}
			if len(result.Errors) > 0 {
				log.Printf("scheduled sync completed with warnings: date=%s warnings=%s", syncDate.Format("2006-01-02"), strings.Join(result.Errors, "; "))
			}
		case <-s.stopCh:
			scheduledTimer.Stop()
			return
		}
	}
}

func parseCronSchedule(expression string) (cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 5 {
		return cronSchedule{}, fmt.Errorf("cron expression must contain 5 fields")
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("parse minute field: %w", err)
	}

	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("parse hour field: %w", err)
	}

	for index, field := range fields[2:] {
		if field != "*" {
			return cronSchedule{}, fmt.Errorf("field %d must be *", index+3)
		}
	}

	return cronSchedule{
		minute: minute,
		hour:   hour,
	}, nil
}

func parseCronField(raw string, minValue, maxValue int) (cronField, error) {
	switch {
	case raw == "*":
		return cronField{kind: cronFieldAny}, nil
	case strings.HasPrefix(raw, "*/"):
		step, err := strconv.Atoi(strings.TrimPrefix(raw, "*/"))
		if err != nil || step <= 0 {
			return cronField{}, fmt.Errorf("invalid step value")
		}
		return cronField{kind: cronFieldStep, value: step}, nil
	default:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cronField{}, fmt.Errorf("invalid field value")
		}
		if value < minValue || value > maxValue {
			return cronField{}, fmt.Errorf("field value out of range")
		}
		return cronField{kind: cronFieldExact, value: value}, nil
	}
}

func (s cronSchedule) Next(from time.Time) time.Time {
	base := from.UTC().Truncate(time.Minute).Add(time.Minute)
	for {
		if s.hour.matches(base.Hour()) && s.minute.matches(base.Minute()) {
			return base
		}
		base = base.Add(time.Minute)
	}
}

func (f cronField) matches(value int) bool {
	switch f.kind {
	case cronFieldAny:
		return true
	case cronFieldExact:
		return value == f.value
	case cronFieldStep:
		return value%f.value == 0
	default:
		return false
	}
}

func toSyncDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
