package watcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// TestResumableWatch_EventsHandled verifies that the callback is invoked for ADDED, MODIFIED, and DELETED events.
func TestResumableWatch_EventsHandled(t *testing.T) {
	ctrl := gomock.NewController(t)
	watcher := watch.NewFake()
	mockClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
	mockClient.EXPECT().Watch(gomock.Any(), gomock.Any()).Return(watcher, nil)

	// This slice will store the events processed by the callback
	var processedEvents []watch.EventType
	callback := func(obj *corev1.ConfigMap, eventType watch.EventType) {
		processedEvents = append(processedEvents, eventType)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go resumableWatch[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, mockClient, callback)

	// Send events to the watcher
	obj := &corev1.ConfigMap{}
	watcher.Add(obj)
	watcher.Modify(obj)
	watcher.Delete(obj)

	// Give the goroutine time to process events
	time.Sleep(100 * time.Millisecond)
	cancel()

	assert.Equal(t, []watch.EventType{watch.Added, watch.Modified, watch.Deleted}, processedEvents)
}

// TestResumableWatch_ContextCanceled ensures the watch loop terminates when the context is canceled.
func TestResumableWatch_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	watcher := watch.NewFake()
	mockClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
	mockClient.EXPECT().Watch(gomock.Any(), gomock.Any()).Return(watcher, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		resumableWatch[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, mockClient, nil)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Watcher was not closed within the specified timeout")
	case <-done:
		assert.True(t, watcher.IsStopped())
	}
}

// TestResumableWatch_RetryAfterError tests if the function retries when watch creation fails.
func TestResumableWatch_RetryAfterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	watcher := watch.NewFake()
	mockClient := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	var attempts int
	mockClient.EXPECT().Watch(gomock.Any(), gomock.Any()).DoAndReturn(func(string, metav1.ListOptions) (watch.Interface, error) {
		attempts++
		// Return an error on the first attempt to watch
		if attempts <= 1 {
			return nil, fmt.Errorf("can't watch this!")
		}
		return watcher, nil
	}).Times(2) // expect to be called twice

	var callbackCalled bool
	callback := func(obj *corev1.Secret, eventType watch.EventType) {
		callbackCalled = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resumableWatch[*corev1.Secret, *corev1.SecretList](ctx, mockClient, callback,
		// Use a very short retry period for the test
		WithRetryPeriod(10*time.Millisecond))

	watcher.Add(&corev1.Secret{})

	// Give it time to fail, retry, and process the event
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 2, attempts, "Expected two watch attempts")
	assert.True(t, callbackCalled, "Callback should have been called after successful retry")
}

// TestResumableWatch_ResumesAfterErrorEvent verifies that the watch is re-established after an interruption.
func TestResumableWatch_ResumesAfterErrorEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	watcher := watch.NewFake()
	mockClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
	var doneFirstWatch bool
	mockClient.EXPECT().Watch(gomock.Any(), gomock.Cond(func(opts metav1.ListOptions) bool {
		// Verify that resumed watches provide a resource version
		return !doneFirstWatch || opts.ResourceVersion == "1"
	})).DoAndReturn(func(string, metav1.ListOptions) (watch.Interface, error) {
		doneFirstWatch = true
		watcher.Reset()
		return watcher, nil
	}).Times(2)

	var eventsProcessed int
	callback := func(obj *corev1.ConfigMap, eventType watch.EventType) {
		eventsProcessed++
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resumableWatch[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, mockClient, callback)
	for !doneFirstWatch {
		time.Sleep(50 * time.Millisecond)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
		},
	}
	watcher.Add(cm)
	watcher.Error(nil)

	// Wait for events to be processed and watch resumed (first try is immediate)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, eventsProcessed, "Expected one watch event")

	cm.ResourceVersion = "2"
	watcher.Modify(cm)

	// Verify the watcher keeps working and processing events after the interruption
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, eventsProcessed, "Expected one watch event")
}

// TestResumableWatch_ResetResourceVersionAfterExpired verifies that the watch is reset after receiving 410 Gone HTTP code
func TestResumableWatch_ResetResourceVersionAfterExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	watcher := watch.NewFake()
	mockClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
	var count int
	mockClient.EXPECT().Watch(gomock.Any(), gomock.Cond(func(opts metav1.ListOptions) bool {
		switch {
		case count == 1:
			return opts.ResourceVersion == "1"
		case count == 2:
			return opts.ResourceVersion == ""
		}
		return true
	})).DoAndReturn(func(_ string, opts metav1.ListOptions) (watch.Interface, error) {
		count++
		watcher.Reset()
		if opts.ResourceVersion == "1" {
			return nil, errors.NewResourceExpired("resource version too old")
		}
		return watcher, nil
	}).Times(3)

	var eventsProcessed int
	callback := func(obj *corev1.ConfigMap, eventType watch.EventType) {
		eventsProcessed++
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resumableWatch[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, mockClient, callback)
	for count == 0 {
		time.Sleep(50 * time.Millisecond)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
		},
	}
	watcher.Add(cm)
	watcher.Error(nil)

	// Wait for events to be processed and watch resumed (first try is immediate)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, eventsProcessed, "Expected one watch event")

	cm.ResourceVersion = "2"
	watcher.Modify(cm)

	// Verify the watcher keeps working and processing events after the interruption
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, eventsProcessed, "Expected one watch event")
}
