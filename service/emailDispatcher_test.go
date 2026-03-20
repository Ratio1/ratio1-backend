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

	const handlerName = "test_drain_pending_tasks"
	var executed atomic.Int32
	registerEmailTaskHandler(handlerName, func(task EmailTask) error {
		time.Sleep(10 * time.Millisecond)
		executed.Add(1)
		return nil
	})

	StartEmailDispatcher(context.TODO())

	const tasksCount = 5
	for i := 0; i < tasksCount; i++ {
		EnqueueEmailTask(EmailTask{
			Name: handlerName,
		}, false)
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

	const firstHandlerName = "test_first_blocking_task"
	const secondHandlerName = "test_second_task_should_be_dropped"
	blockTask := make(chan struct{})
	firstStarted := make(chan struct{})

	var firstExecuted atomic.Bool
	var secondExecuted atomic.Bool

	registerEmailTaskHandler(firstHandlerName, func(task EmailTask) error {
		firstExecuted.Store(true)
		close(firstStarted)
		<-blockTask
		return nil
	})
	registerEmailTaskHandler(secondHandlerName, func(task EmailTask) error {
		secondExecuted.Store(true)
		return nil
	})

	StartEmailDispatcher(context.TODO())

	EnqueueEmailTask(EmailTask{
		Name: firstHandlerName,
	}, false)

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
		Name: secondHandlerName,
	}, false)

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
	emailDispatcher.nodeAddr = ""
	resetEmailTaskHandlersForTest()
}
