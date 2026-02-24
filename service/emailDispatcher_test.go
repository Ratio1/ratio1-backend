package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStopEmailDispatcherDrainsPendingTasks(t *testing.T) {
	resetEmailDispatcherForTest()
	t.Cleanup(cleanupEmailDispatcherForTest)

	StartEmailDispatcher(nil)

	const tasksCount = 5
	var executed atomic.Int32
	for i := 0; i < tasksCount; i++ {
		EnqueueEmailTask(EmailTask{
			Name: "drain_pending_task",
			Execute: func() error {
				time.Sleep(10 * time.Millisecond)
				executed.Add(1)
				return nil
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	StopEmailDispatcher(ctx)

	if got := int(executed.Load()); got != tasksCount {
		t.Fatalf("expected %d executed tasks, got %d", tasksCount, got)
	}
}

func TestEnqueueEmailTaskDropsTasksWhenStopping(t *testing.T) {
	resetEmailDispatcherForTest()
	t.Cleanup(cleanupEmailDispatcherForTest)

	StartEmailDispatcher(nil)

	blockTask := make(chan struct{})
	firstStarted := make(chan struct{})

	var firstExecuted atomic.Bool
	var secondExecuted atomic.Bool

	EnqueueEmailTask(EmailTask{
		Name: "first_blocking_task",
		Execute: func() error {
			firstExecuted.Store(true)
			close(firstStarted)
			<-blockTask
			return nil
		},
	})

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the first task to start")
	}

	stopDone := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		StopEmailDispatcher(ctx)
		close(stopDone)
	}()

	waitForDispatcherStopping(t)

	EnqueueEmailTask(EmailTask{
		Name: "second_task_should_be_dropped",
		Execute: func() error {
			secondExecuted.Store(true)
			return nil
		},
	})

	close(blockTask)

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatcher stop")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	StopEmailDispatcher(ctx)

	if !firstExecuted.Load() {
		t.Fatal("expected first task to execute")
	}
	if secondExecuted.Load() {
		t.Fatal("expected second task to be dropped while dispatcher is stopping")
	}
}

func waitForDispatcherStopping(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		emailDispatcher.mu.Lock()
		stopping := emailDispatcher.stopping
		emailDispatcher.mu.Unlock()
		if stopping {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for email dispatcher to enter stopping state")
}

func cleanupEmailDispatcherForTest() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	StopEmailDispatcher(ctx)
	resetEmailDispatcherForTest()
}

func resetEmailDispatcherForTest() {
	emailDispatcher.mu.Lock()
	defer emailDispatcher.mu.Unlock()

	emailDispatcherOnce = sync.Once{}
	emailDispatcher.queue = nil
	emailDispatcher.stopping = false
	emailDispatcher.done = nil
}
