package jitterbug

// jitterbug is a package to implement a lightweight jitter based task system
// The key principal

import (
	"time"

	"github.com/sirupsen/logrus"
	_ "net/http/pprof"
)

type JitterFunction func(dailyTriggerTime, maxTriggerTime time.Duration) (bool, error)

type JitterChecker struct {
	config               *Config
	calculator           Calculator
	callable             JitterFunction
	ticker               *time.Ticker
	dailyCheckinInterval time.Duration
}

// NewJitterChecker will complete initialization of optional Config fields and return a jitter checker
func NewJitterChecker(config *Config, callable JitterFunction) *JitterChecker {
	calculator := NewJitterCalculator(config, nil)
	return &JitterChecker{
		config:     config,
		calculator: calculator,
		callable:   callable,
	}
}

// NewJitterCheckerFromCalculator will complete initialization of optional Config fields and return a jitter checker
func NewJitterCheckerFromCalculator(calculator JitterCalculator, callable JitterFunction) *JitterChecker {
	return &JitterChecker{
		config:     calculator.config,
		calculator: &calculator,
		callable:   callable,
	}
}

// Start prepares the first checkin interval and starts the ticker
func (j *JitterChecker) Start() {
	j.calculateCheckinInterval()
	j.ticker = time.NewTicker(j.config.PollingInterval)
}

func (j *JitterChecker) calculateCheckinInterval() {
	j.dailyCheckinInterval = j.calculator.CalculateCheckinInterval()
}

func (j *JitterChecker) Run() {
	defer j.ticker.Stop()
	for range j.ticker.C {
		logrus.Debugf("JitterChecker Run: tick")
		j.run()
	}
}

func (j *JitterChecker) run() {
	// Apply initial delay if configured
	if j.config.InitialDelay > 0 {
		logrus.Debugf("[Checker] Initial delay of %s...\n", j.config.InitialDelay)
		select {
		case <-time.After(j.config.InitialDelay):
			// Proceed
		}
	}
	refresh, err := j.callable(j.dailyCheckinInterval, j.config.StrictDeadline)
	if err != nil {
		logrus.Errorf("JitterChecker Run-error: %v", err)
		return
	}

	if refresh {
		j.calculateCheckinInterval()
	}
}
