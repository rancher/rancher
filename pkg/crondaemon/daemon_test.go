package crondaemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron"
)

type fakeCronRunner struct {
	ch      chan time.Time
	done    chan struct{}
	job     cron.Job
	running bool
}

func (f *fakeCronRunner) Start() {
	f.done = make(chan struct{})
	go func() {
		for {
			select {
			case <-f.ch:
				if f.job != nil {
					go f.job.Run()
				}
			case <-f.done:
				return
			}
		}
	}()
	f.running = true
}

func (f *fakeCronRunner) Stop() {
	if f.running {
		close(f.done)
		f.running = false
	}
}

func (f *fakeCronRunner) Schedule(_ cron.Schedule, job cron.Job) {
	f.job = job
}

func (f *fakeCronRunner) Wait() {
	<-f.done
}

func TestNew(t *testing.T) {
	// This is a slow test that exercises the real cron runner.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ran atomic.Uint32

	daemon := New(ctx, "testdaemon", func(ctx context.Context) error {
		ran.Add(1)
		return nil
	})

	err := daemon.Schedule("@every 1s")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second + 250*time.Millisecond)
	if ran.Load() < 1 {
		t.Errorf("Expected to run at least once")
	}
}

func TestScheduleAndDisable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		ran        atomic.Uint32
		fakeRunner *fakeCronRunner
	)

	timeCh := make(chan time.Time)
	defer close(timeCh)

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			fakeRunner = &fakeCronRunner{ch: timeCh}
			return fakeRunner
		},
		run: func(ctx context.Context) error {
			ran.Add(1)
			return nil
		},
	}

	err := daemon.Schedule("") // This should have no effect.
	if err != nil {
		t.Fatal(err)
	}
	if fakeRunner != nil {
		t.Fatal("Expected runner to not be running")
	}

	err = daemon.Schedule("@every 1s")
	if err != nil {
		t.Fatal(err)
	}
	if fakeRunner == nil {
		t.Fatal("Expected runner to not be nil")
	}
	if !fakeRunner.running {
		t.Fatal("Expected runner to be running")
	}

	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)
	if ran.Load() < 1 {
		t.Errorf("Expected to run at least once")
	}

	err = daemon.Schedule("@every 2s")
	if err != nil {
		t.Fatal(err)
	}
	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)
	if ran.Load() < 2 {
		t.Errorf("Expected to run at least twice")
	}

	err = daemon.Schedule("")
	if err != nil {
		t.Fatal(err)
	}

	fakeRunner.Wait()
	if fakeRunner.running {
		t.Fatal("Expected runner to be stopped")
	}
}

func TestScheduleAndCancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		ran        atomic.Uint32
		fakeRunner *fakeCronRunner
	)

	timeCh := make(chan time.Time)
	defer close(timeCh)

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			fakeRunner = &fakeCronRunner{ch: timeCh}
			return fakeRunner
		},
		run: func(ctx context.Context) error {
			ran.Add(1)
			return nil
		},
	}

	daemon.Schedule("@every 1s")
	if fakeRunner == nil {
		t.Fatal("Expected runner to not be nil")
	}

	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)
	if ran.Load() < 1 {
		t.Errorf("Expected to run at least once")
	}

	cancel()

	fakeRunner.Wait()
	if fakeRunner.running {
		t.Fatal("Expected runner to be stopped")
	}
}

func TestConcurrentRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		ran        atomic.Uint32
		fakeRunner *fakeCronRunner
	)

	timeCh := make(chan time.Time)
	defer close(timeCh)

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			fakeRunner = &fakeCronRunner{ch: timeCh}
			return fakeRunner
		},
		run: func(ctx context.Context) error {
			ran.Add(1)
			time.Sleep(250 * time.Millisecond)
			return nil
		},
	}

	daemon.Schedule("@every 1s")
	if fakeRunner == nil {
		t.Fatal("Expected runner to not be nil")
	}

	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)

	timeCh <- time.Now()
	time.Sleep(600 * time.Millisecond)

	if want, got := uint32(1), ran.Load(); want != got {
		t.Errorf("Expected to run %d got %d", want, got)
	}
}

func TestScheduleWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			t.Fatal("Unexpected call to newRunner")
			return nil
		},
	}

	daemon.Schedule("@every 1s")
}

func TestScheduleWithInvalidCronExpression(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			t.Fatal("Unexpected call to newRunner")
			return nil
		},
	}

	err := daemon.Schedule("foo")
	if err == nil {
		t.Error("Expected error")
	}
}

func TestRescheduleWithInvalidCronExpression(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		ran        atomic.Uint32
		fakeRunner *fakeCronRunner
	)

	timeCh := make(chan time.Time)
	defer close(timeCh)

	daemon := &Daemon{
		ctx: ctx,
		newRunner: func() runner {
			fakeRunner = &fakeCronRunner{ch: timeCh}
			return fakeRunner
		},
		run: func(ctx context.Context) error {
			ran.Add(1)
			return nil
		},
	}

	daemon.Schedule("@every 1s")
	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)

	err := daemon.Schedule("foo") // This should have no effect.
	if err == nil {
		t.Error("Expected error")
	}

	timeCh <- time.Now()
	time.Sleep(100 * time.Millisecond)

	if ran.Load() < 2 {
		t.Errorf("Expected to run at least twice")
	}
}

func TestParseCron(t *testing.T) {
	schedule, err := ParseCron("")
	if err != nil {
		t.Fatal(err)
	}
	if schedule != nil {
		t.Fatal("Expected schedule to be nil")
	}

	schedule, err = ParseCron("0 0 * * *")
	if err != nil {
		t.Fatal(err)
	}
	if schedule == nil {
		t.Fatal("Expected schedule not be be nil")
	}

	schedule, err = ParseCron("invalid")
	if err == nil {
		t.Fatal("Expected error")
	}
	if schedule != nil {
		t.Fatal("Expected schedule to be nil")
	}
}
