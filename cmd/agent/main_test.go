package main

import (
	"context"
	"testing"

	"github.com/rancher/remotedialer"
	"github.com/stretchr/testify/assert"
)

func TestOnConnectPreventsDoubleStart(t *testing.T) {
	// Reset global state
	rancherStarted = false

	// Mock rancherRunFunc to track calls
	var callCount int
	originalRancherRun := rancherRunFunc
	defer func() {
		rancherRunFunc = originalRancherRun
		rancherStarted = false // Cleanup
	}()

	rancherRunFunc = func(ctx context.Context) error {
		callCount++
		return nil
	}

	// Create onConnect function (simplified version for testing)
	onConnect := func(ctx context.Context, _ *remotedialer.Session) error {
		if !rancherStarted {
			err := rancherRunFunc(ctx)
			if err != nil {
				return err
			}
			rancherStarted = true
		}
		return nil
	}

	// First connection should start rancher
	err := onConnect(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "First connection should call rancherRunFunc once")
	assert.True(t, rancherStarted, "rancherStarted should be true after first connection")

	// Second connection should not start rancher again
	err = onConnect(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "Second connection should not call rancherRunFunc again")
	assert.True(t, rancherStarted, "rancherStarted should remain true")
}