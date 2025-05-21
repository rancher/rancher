package jitterbug

// jitterbug is a package to implement a lightweight jitter based task system

import (
	"github.com/sirupsen/logrus"
	"math/rand"
	"time"
)

type JitterFunction func(dailyTriggerTime time.Duration) (bool, error)

type Config struct {
	BaseInterval    time.Duration
	JitterMax       int
	JitterMaxScale  time.Duration
	PollingInterval time.Duration
	InitialDelay    time.Duration
}

type JitterChecker struct {
	config               Config
	callable             JitterFunction
	ticker               *time.Ticker
	dailyCheckinInterval time.Duration
}

func NewJitterChecker(config Config, callable JitterFunction) *JitterChecker {
	if config.BaseInterval == 0 {
		panic("BaseInterval can't be zero")
	}
	if config.PollingInterval == 0 {
		config.PollingInterval = 30 * time.Second
	}

	return &JitterChecker{
		config:   config,
		callable: callable,
	}
}

func (j *JitterChecker) Start() {
	j.calculateCheckinInterval()
	j.ticker = time.NewTicker(j.config.PollingInterval)
	//defer j.ticker.Stop()
}

func (j *JitterChecker) calculateCheckinInterval() {
	jitterValue := rand.Intn(j.config.JitterMax)
	jitterTime := time.Duration(jitterValue) * j.config.JitterMaxScale
	jitterDirection := rand.Intn(2)
	if jitterDirection >= 1 {
		jitterTime *= -1
	}
	j.dailyCheckinInterval = j.config.BaseInterval + jitterTime
}

func (j *JitterChecker) Run() {
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
	refresh, err := j.callable(j.dailyCheckinInterval)
	if err != nil {
		logrus.Errorf("JitterChecker Run-error: %v", err)
		return
	}

	if refresh {
		j.calculateCheckinInterval()
	}
}
