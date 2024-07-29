package crondaemon

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
)

// runner defines the cron runner.
type runner interface {
	Start()
	Stop()
	Schedule(cron.Schedule, cron.Job)
}

// RunFunc is the function that is run by the daemon on the schedule.
type RunFunc func(context.Context) error

// Daemon runs a runFunc on a cron schedule.
type Daemon struct {
	mu            sync.Mutex
	ctx           context.Context // The context to pass on each run invocation.
	name          string          // The name of the daemon.
	run           RunFunc         // The function to run on the cron schedule.
	runner        runner          // The cron runner instance.
	newRunner     func() runner   // The function that instantiates a new cron runner.
	lastExp       string          // The last successfully applied cron expression.
	running       bool            // True if the daemon was successfully scheduled and is running.
	everScheduled bool            // True if the daemon was ever scheduled.
	done          chan struct{}   // The channel that is closed when the daemon is disabled by passing an empty cron expression.
	runInProgress atomic.Bool     // True if the run function is currently running.
}

// New creates a new daemon instance.
func New(ctx context.Context, name string, run RunFunc) *Daemon {
	return &Daemon{
		ctx:  ctx,
		name: name,
		run:  run,
		newRunner: func() runner {
			return cron.New()
		},
	}
}

// Schedule the daemon with the new cron expression.
// If the cron expression is empty, the daemon is stopped/disabled.
// Subsequent calls with the same cron expression are no-op.
// The daemon logs initial and all effective schedule calls using info level
// and errors returned by the run function using error level.
func (d *Daemon) Schedule(exp string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ctx.Err() != nil {
		return nil
	}

	if (!d.everScheduled || d.running) && exp == "" {
		d.everScheduled = true
		logrus.Info(d.withPrefix("daemon is disabled"))
	}

	if d.running && exp == d.lastExp ||
		!d.running && exp == "" {
		return nil
	}

	if exp == "" {
		if d.runner != nil {
			d.runner.Stop()
		}
		close(d.done)
		d.lastExp = ""
		d.running = false

		return nil
	}

	schedule, err := ParseCron(exp)
	if err != nil {
		return err
	}

	d.lastExp = exp
	d.everScheduled = true

	logrus.Info(d.withPrefix("daemon is scheduled with '" + exp + "'"))

	if d.runner != nil {
		d.runner.Stop()
	}
	d.runner = d.newRunner()
	d.runner.Schedule(schedule, cron.FuncJob(func() {
		if d.runInProgress.Load() {
			return // Skip if it the previous run is still in progress.
		}
		d.runInProgress.Store(true)
		defer d.runInProgress.Store(false)

		if err := d.run(d.ctx); err != nil {
			logrus.Error(d.withPrefix(err.Error()))
		}
	}))
	d.runner.Start()

	d.done = make(chan struct{})
	if !d.running {
		go func() {
			select {
			case <-d.done:
				return
			case <-d.ctx.Done():
				logrus.Info(d.withPrefix("context cancelled, stopping daemon"))

				d.mu.Lock()
				defer d.mu.Unlock()
				if d.runner != nil {
					d.runner.Stop()
				}
			}
		}()
	}
	d.running = true

	return nil
}

// withPrefix is a helper that adds the daemon name to log messages.
func (d *Daemon) withPrefix(msg string) string {
	if d.name == "" {
		return msg
	}

	return d.name + ": " + msg
}

// ParseCron parses a cron expression.
func ParseCron(exp string) (cron.Schedule, error) {
	if exp == "" {
		return nil, nil
	}

	schedule, err := cron.ParseStandard(exp)
	if err != nil {
		return nil, fmt.Errorf("error parsing cron: %w", err)
	}

	return schedule, nil
}
