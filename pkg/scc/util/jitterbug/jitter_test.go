package jitterbug

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"math/rand"
	"testing"
	"time"
)

// MockJitterCalculator is a mock implementation of JitterCalculator for testing purposes.
type MockJitterCalculator struct {
	mock.Mock
	config *Config
}

func (m *MockJitterCalculator) CalculateCheckinInterval() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

// Helper to create a basic config for testing
func createTestConfig() Config {
	config := Config{
		BaseInterval:    3 * time.Second,
		JitterMax:       500,
		JitterMaxScale:  time.Millisecond,
		PollingInterval: time.Second, // Small interval for faster tests
	}

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	NewJitterCalculator(&config, r)

	return config
}

func TestJitterChecker_Start(t *testing.T) {
	config := createTestConfig()

	mockCalculator := new(MockJitterCalculator)
	mockCalculator.config = &config
	mockCalculator.On("CalculateCheckinInterval").Return(5 * time.Second)

	jc := &JitterChecker{
		config:     mockCalculator.config,
		calculator: mockCalculator,
		callable: func(_, _ time.Duration) (bool, error) {
			return false, nil
		},
	}

	jc.Start()

	// Assert that the calculated interval was stored
	assert.Equal(t, 5*time.Second, jc.triggerInterval)

	// Assert that the tickChan is not nil
	assert.NotNil(t, jc.tickChan)

	// We can't assert much else about tickChan from time.Tick() (it's a global timer),
	// but we can validate that it's set.
	mockCalculator.AssertExpectations(t)
}

func TestJitterChecker_Run(t *testing.T) {
	config := createTestConfig()
	tickChan := make(chan time.Time)

	var callCount, triggerCount int
	var elapsed time.Duration

	mockCallable := func(daily, max time.Duration) (bool, error) {
		callCount++
		if elapsed > daily || elapsed > max {
			triggerCount++
			elapsed = 0
			return true, nil // Trigger interval change
		}
		return false, nil // Don't trigger interval change
	}

	// Use a deterministic random source
	seed := int64(42)
	r := rand.New(rand.NewSource(seed))

	jc := &JitterChecker{
		log:        jitterbugContextLogger(),
		config:     &config,
		calculator: NewJitterCalculator(&config, r),
		callable:   mockCallable,
		tickChan:   tickChan, // inject test-controlled tickChan
	}

	// Run JitterChecker in a goroutine so we can send ticks
	jc.Start()

	go jc.Run()

	// Simulate ticks and manual time progression
	for i := 0; i < 6; i++ {
		elapsed += 2 * time.Second // enough to sometimes cross interval thresholds
		tickChan <- time.Now()
		time.Sleep(10 * time.Millisecond)
	}

	close(tickChan)

	assert.Equal(t, 6, callCount)
	assert.GreaterOrEqual(t, triggerCount, 1)
}

func TestJitterChecker_RunIntervalChanged(t *testing.T) {
	config := createTestConfig()
	tickChan := make(chan time.Time)

	calculatedIntervals := []time.Duration{
		5 * time.Second,
		10 * time.Second,
	}

	// Capture assigned intervals
	var assignedIntervals []time.Duration

	// Mock the callable to trigger an interval change on the first tick only
	callCount := 0
	mockCallable := func(daily, max time.Duration) (bool, error) {
		callCount++
		assignedIntervals = append(assignedIntervals, daily)

		// Trigger interval change only on the first call
		if callCount == 1 {
			return true, nil
		}
		return false, nil
	}

	// Mock the calculator to return different values on each call
	mockCalculator := &MockJitterCalculator{}
	mockCalculator.config = &config
	mockCalculator.On("CalculateCheckinInterval").Return(calculatedIntervals[0]).Once()
	mockCalculator.On("CalculateCheckinInterval").Return(calculatedIntervals[1]).Once()

	jc := &JitterChecker{
		log:        jitterbugContextLogger(),
		config:     &config,
		calculator: mockCalculator,
		callable:   mockCallable,
		tickChan:   tickChan,
	}

	jc.Start()
	go jc.Run()

	// Simulate two ticks: one that triggers interval change, one that doesn't
	tickChan <- time.Now()
	time.Sleep(50 * time.Millisecond)
	tickChan <- time.Now()
	time.Sleep(50 * time.Millisecond)

	// Close the tick channel to stop the goroutine
	close(tickChan)

	// Assertions
	assert.Equal(t, 2, callCount)
	assert.Equal(t, 2, len(assignedIntervals))
	assert.Equal(t, calculatedIntervals[0], assignedIntervals[0])
	assert.Equal(t, calculatedIntervals[1], assignedIntervals[1])
	mockCalculator.AssertExpectations(t)
}
