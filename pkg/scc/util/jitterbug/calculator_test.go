package jitterbug

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func TestNewJitterCalculator(t *testing.T) {
	t.Parallel()
	config := &Config{
		BaseInterval:   10 * time.Hour,
		JitterMax:      5,
		JitterMaxScale: time.Minute,
	}
	jc := NewJitterCalculator(config, nil)
	if jc.config.BaseInterval != config.BaseInterval {
		t.Errorf("BaseInterval not set correctly")
	}
	if jc.config.PollingInterval != config.PollingInterval {
		t.Errorf("PollingInterval not set correctly")
	}
	if jc.config.JitterMax != config.JitterMax {
		t.Errorf("JitterMax not set correctly")
	}
	if jc.config.JitterMaxScale != config.JitterMaxScale {
		t.Errorf("JitterMaxScale not set correctly")
	}
	if jc.config.StrictDeadline < (config.BaseInterval + config.JitterMaxDuration()) {
		t.Errorf("StrictDeadline not calculated correctly")
	}
}

func TestNewJitterCalculatorDefaults(t *testing.T) {
	t.Parallel()
	config := &Config{
		BaseInterval: 10 * time.Hour,
	}
	_ = NewJitterCalculator(config, nil)
	assert.Equal(t, 30*time.Second, config.PollingInterval)
	assert.Equal(t, 15, config.JitterMax)
	assert.Equal(t, time.Minute, config.JitterMaxScale)
	assert.Equal(t, config.BaseInterval+config.JitterMaxDuration(), config.StrictDeadline)
}

func TestCalculateCheckinInterval(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"42", 42, 8},
		{"32", 32, 12},
		{"9", 9, 9},
		{"1024", 1024, 10},
		{"2048", 2048, 12},
	}

	config := &Config{
		BaseInterval:   10 * time.Second,
		JitterMax:      2,
		JitterMaxScale: time.Second,
	}

	for _, tt := range tests {
		// Intentionally set a local var to ensure the parallel calls don't collide
		localTT := tt
		t.Run(localTT.name, func(t *testing.T) {
			t.Parallel()
			// Use a deterministic random source
			seed := int64(localTT.input)
			r := rand.New(rand.NewSource(seed))

			calculator := NewJitterCalculator(config, r)
			interval := calculator.CalculateCheckinInterval()

			expected := time.Duration(localTT.expected) * config.JitterMaxScale
			assert.Equal(t, expected, interval)
		})
	}
}

func TestCalculateCheckinIntervalAltConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    int
		expected time.Duration
	}{
		{"42", 42, (12 * time.Hour) + (9 * time.Minute)},
		{"32", 32, (9 * time.Hour) + (17 * time.Minute)},
		{"9", 9, (7 * time.Hour) + (6 * time.Minute)},
		{"1024", 1024, (10 * time.Hour) + (8 * time.Minute)},
		{"2048", 2048, (10 * time.Hour) + (35 * time.Minute)},
	}

	config := &Config{
		BaseInterval:   10 * time.Hour,
		JitterMax:      int(3 * time.Hour / time.Minute),
		JitterMaxScale: time.Minute,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a deterministic random source
			seed := int64(tt.input)
			r := rand.New(rand.NewSource(seed))

			calculator := NewJitterCalculator(config, r)
			interval := calculator.CalculateCheckinInterval()

			assert.Equal(t, tt.expected, interval)
		})
	}
}

func TestIncorrectConfig(t *testing.T) {
	t.Parallel()
	config := &Config{}
	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	NewJitterCalculator(config, r)

	// If the panic was not caught, the test will fail
	t.Errorf("Test failed, panic was expected")
}

func TestCalculateCheckinInterval_with_negative_jitter(t *testing.T) {
	t.Parallel()
	config := &Config{
		BaseInterval:    10,
		PollingInterval: 30 * time.Second,
		JitterMax:       5,
		JitterMaxScale:  time.Minute,
	}

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	defer func() {
		if r := recover(); r != nil {
			// We successfully recovered from panic
			t.Log("Test passed, panic was caught!")
		}
	}()

	_ = NewJitterCalculator(config, r)

	// If the panic was not caught, the test will fail
	t.Errorf("Test failed, panic was expected")
}
