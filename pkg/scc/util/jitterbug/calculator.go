package jitterbug

import (
	"math/rand"
	"time"
)

// Config holds the config for the jitter checker
// the only required field is BaseInterval
type Config struct {
	BaseInterval    time.Duration
	JitterMax       int
	JitterMaxScale  time.Duration
	StrictDeadline  time.Duration
	PollingInterval time.Duration
	InitialDelay    time.Duration
}

func (c *Config) JitterMaxDuration() time.Duration {
	return time.Duration(c.JitterMax) * c.JitterMaxScale
}

type JitterCalculator struct {
	config *Config
	rand   *rand.Rand
}

type Calculator interface {
	CalculateCheckinInterval() time.Duration
}

// NewJitterCalculator will complete initialization of optional Config fields and return a JitterCalculator
func NewJitterCalculator(config *Config, r *rand.Rand) *JitterCalculator {
	if config.BaseInterval == 0 {
		panic("BaseInterval can't be zero")
	}
	if config.PollingInterval == 0 {
		config.PollingInterval = 30 * time.Second
	}
	if config.JitterMax == 0 {
		config.JitterMax = 15
	}
	if config.JitterMaxScale == 0 {
		config.JitterMaxScale = time.Minute
	}
	if config.StrictDeadline == 0 {
		config.StrictDeadline = config.BaseInterval + config.JitterMaxDuration()
	}

	if config.BaseInterval-config.JitterMaxDuration() < 0 {
		panic("BaseInterval must be greater than JitterMaxDuration; otherwise jitter randomization could cause negative time interval")
	}

	// Set randomness when not provided, this is the default and overriding is mainly for testing purposes.
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return &JitterCalculator{
		config: config,
		rand:   r,
	}
}

// calculateRandomJitter will generate a random jitter value within the specified range.
func (jc *JitterCalculator) calculateRandomJitter() int {
	maxJitter := jc.config.JitterMax
	// Generate a value between 0 and 2*maxJitter
	jitterValue := jc.rand.Intn(maxJitter*2 + 1)
	// Adjust the value to be within the range of -maxJitter to maxJitter
	return jitterValue - maxJitter
}

// CalculateCheckinInterval will calculate a random interval using the provided jitter configuration.
func (jc *JitterCalculator) CalculateCheckinInterval() time.Duration {
	jitterValue := jc.calculateRandomJitter()
	jitterTime := time.Duration(jitterValue) * jc.config.JitterMaxScale

	return jc.config.BaseInterval + jitterTime
}
