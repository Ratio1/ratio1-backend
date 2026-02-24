package service

import (
	"context"
	"fmt"
	"sync"
)

const defaultEmailQueueSize = 10000

// EmailTask is a single mail operation scheduled by API/cron flows.
// The dispatcher executes Execute() and logs failures using Name as identifier.
type EmailTask struct {
	Name    string
	Execute func() error
}

var (
	emailDispatcherOnce sync.Once
	emailDispatcher     dispatcherState
)

type dispatcherState struct {
	mu       sync.Mutex
	queue    chan EmailTask
	done     chan struct{}
	stopping bool
}

func StartEmailDispatcher(ctx context.Context) {
	// Startup is idempotent: only one queue + one worker goroutine are created.
	emailDispatcherOnce.Do(func() {
		queue := make(chan EmailTask, defaultEmailQueueSize)
		done := make(chan struct{})

		emailDispatcher.mu.Lock()
		emailDispatcher.queue = queue
		emailDispatcher.done = done
		emailDispatcher.stopping = false
		emailDispatcher.mu.Unlock()

		go emailDispatcherLoop(queue, done)

		if ctx == nil {
			return
		}
		stopSignal := ctx.Done()
		if stopSignal == nil {
			return
		}
		go func() {
			<-stopSignal
			stopEmailDispatcher()
		}()
	})
}

// StopEmailDispatcher prevents new enqueues, closes the queue, and waits for the worker to stop.
func StopEmailDispatcher(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	stopEmailDispatcher()
	waitForDispatcherDone(ctx)
}

func stopEmailDispatcher() {
	emailDispatcher.mu.Lock()
	defer emailDispatcher.mu.Unlock()

	if emailDispatcher.queue == nil {
		return
	}
	if emailDispatcher.stopping {
		return
	}
	emailDispatcher.stopping = true
	close(emailDispatcher.queue)
	emailDispatcher.queue = nil
}

func EnqueueEmailTask(task EmailTask) {
	if task.Execute == nil {
		log.Error("email task has nil executor: %s", task.Name)
		return
	}

	emailDispatcher.mu.Lock()
	queue := emailDispatcher.queue
	stopping := emailDispatcher.stopping
	// Dispatcher startup is explicit in bootstrap (cmd/main.go).
	// Fail fast if someone enqueues before initialization.
	if queue == nil {
		emailDispatcher.mu.Unlock()
		if stopping {
			log.Warn("email dispatcher is stopping, dropping task: %s", task.Name)
			return
		}
		panic("email dispatcher not started")
	}
	if stopping {
		emailDispatcher.mu.Unlock()
		log.Warn("email dispatcher is stopping, dropping task: %s", task.Name)
		return
	}
	emailDispatcher.mu.Unlock()

	// Sending to the channel queues the task for the background worker.
	// When the buffer is full, this blocks until the worker consumes tasks.
	defer func() {
		if rec := recover(); rec != nil {
			log.Warn("email dispatcher queue closed while enqueueing task (%s): %v", task.Name, rec)
		}
	}()
	//todo use a different system switch->case with timeout to avoid blocking indefinitely when the queue is full?
	//also implement a caching system with cstore to store failed tasks and retry later?
	queue <- task
}

func emailDispatcherLoop(queue <-chan EmailTask, done chan struct{}) {
	defer close(done)
	// The loop waits for incoming tasks and runs until the queue channel is closed.
	for task := range queue {
		if err := runEmailTask(task); err != nil {
			log.Error("email task failed (%s): %v", task.Name, err)
		}
	}
}

func waitForDispatcherDone(ctx context.Context) {
	emailDispatcher.mu.Lock()
	done := emailDispatcher.done
	emailDispatcher.mu.Unlock()

	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
		log.Warn("email dispatcher worker did not stop before shutdown deadline")
	}
}

func runEmailTask(task EmailTask) (err error) {
	// Recover from task panics so a single bad task cannot kill the dispatcher worker.
	//TODO different system with cache or DB to store failed tasks and retry later?
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic while processing email task %q: %v", task.Name, rec)
		}
	}()

	return task.Execute()
}
