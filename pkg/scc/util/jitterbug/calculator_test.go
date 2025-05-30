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

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	jc := NewJitterCalculator(config, r)
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

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	_ = NewJitterCalculator(config, r)
	assert.Equal(t, 30*time.Second, config.PollingInterval)
	assert.Equal(t, 15, config.JitterMax)
	assert.Equal(t, time.Minute, config.JitterMaxScale)
	assert.Equal(t, config.BaseInterval+config.JitterMaxDuration(), config.StrictDeadline)
}

// TestCalculateCheckinInterval tests the CalculateCheckinInterval method with the new signature.
// This test was failing due to data races because its subtests were implicitly relying on
// a shared random source. By providing a unique rand.Rand instance per subtest,
// we eliminate this race.
func TestCalculateCheckinInterval(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "Typical interval 42s",
			config: Config{
				BaseInterval:   42 * time.Second,
				JitterMax:      5,
				JitterMaxScale: time.Second, // Jitter range: -5s to +5s
			},
		},
		{
			name: "Typical interval 32s",
			config: Config{
				BaseInterval:   32 * time.Second,
				JitterMax:      10,
				JitterMaxScale: time.Second, // Jitter range: -10s to +10s
			},
		},
		{
			name: "Small interval 9s",
			config: Config{
				BaseInterval:   9 * time.Second,
				JitterMax:      2,
				JitterMaxScale: time.Second, // Jitter range: -2s to +2s
			},
		},
		{
			name: "Large interval 1024s",
			config: Config{
				BaseInterval:   1024 * time.Second,
				JitterMax:      60,
				JitterMaxScale: time.Second, // Jitter range: -60s to +60s
			},
		},
		{
			name: "Large interval 2048s",
			config: Config{
				BaseInterval:   2048 * time.Second,
				JitterMax:      120,
				JitterMaxScale: time.Second, // Jitter range: -120s to +120s
			},
		},
		{
			name: "Zero jitter",
			config: Config{
				BaseInterval:   50 * time.Second,
				JitterMax:      0,
				JitterMaxScale: time.Second, // Jitter range: 0s
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a deterministic random source
			seed := int64(42)
			r := rand.New(rand.NewSource(seed))
			calc := NewJitterCalculator(&tt.config, r)
			if calc == nil {
				t.Fatalf("NewJitterCalculator returned nil")
			}

			// Calculate expected min and max based on the new jitter logic.
			expectedMin := tt.config.BaseInterval - tt.config.JitterMaxDuration()
			expectedMax := tt.config.BaseInterval + tt.config.JitterMaxDuration()

			const iterations = 1000
			for i := 0; i < iterations; i++ {
				result := calc.CalculateCheckinInterval()
				if result < expectedMin || result > expectedMax {
					t.Errorf("Iteration %d: Calculated interval %v out of expected range [%v, %v]", i, result, expectedMin, expectedMax)
				}
			}
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
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "BaseInterval is zero",
			config: Config{
				BaseInterval:   0,
				JitterMax:      1,
				JitterMaxScale: time.Second,
			},
		},
		{
			name: "BaseInterval less than JitterMaxDuration",
			config: Config{
				BaseInterval:   5 * time.Second,
				JitterMax:      10,
				JitterMaxScale: time.Second, // JitterMaxDuration = 10s, 5s - 10s < 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic for config: %+v, but did not panic", tt.config)
				}
			}()
			// Use a deterministic random source
			seed := int64(42)
			r := rand.New(rand.NewSource(seed))
			_ = NewJitterCalculator(&tt.config, r)
		})
	}
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
