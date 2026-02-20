package service

import (
	"fmt"
	"sync"
)

const defaultEmailQueueSize = 256

// EmailTask is a single mail operation scheduled by API/cron flows.
// The dispatcher executes Execute() and logs failures using Name as identifier.
type EmailTask struct {
	Name    string
	Execute func() error
}

var (
	emailDispatcherOnce sync.Once
	emailTaskQueue      chan EmailTask
)

func StartEmailDispatcher() {
	// Startup is idempotent: only one queue + one worker goroutine are created.
	emailDispatcherOnce.Do(func() {
		emailTaskQueue = make(chan EmailTask, defaultEmailQueueSize)
		go emailDispatcherLoop()
	})
}

func EnqueueEmailTask(task EmailTask) {
	if task.Execute == nil {
		log.Error("email task has nil executor: %s", task.Name)
		return
	}
	// Dispatcher startup is explicit in bootstrap (cmd/main.go).
	// Fail fast if someone enqueues before initialization.
	if emailTaskQueue == nil {
		panic("email dispatcher not started")
	}

	// Sending to the channel queues the task for the background worker.
	// When the buffer is full, this blocks until the worker consumes tasks.
	emailTaskQueue <- task
}

func emailDispatcherLoop() {
	// The loop waits for incoming tasks and runs until the queue channel is closed.
	for task := range emailTaskQueue {
		if err := runEmailTask(task); err != nil {
			log.Error("email task failed (%s): %v", task.Name, err)
		}
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
