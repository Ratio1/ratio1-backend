package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/google/uuid"
)

const defaultEmailQueueSize = 10000
const defaultEmailTaskRetryLimit = 3
const defaultEmailTaskRetryWindow = 3 * time.Hour
const defaultEmailTaskStuckThreshold = 3 * time.Hour
const defaultEmailTaskRetryCooldown = 5 * time.Minute
const defaultProceedingTasksHashKey = "ratio1_email_tasks_proceeding"
const defaultFailedTasksHashKey = "ratio1_email_tasks_failed"
const defaultFinalFailedTasksHashKey = "ratio1_email_tasks_final_failed"

// EmailTask is a serializable email job executed via the handler registry.
// Persisted tasks are written to cstore only for retry/recovery flows.
type EmailTask struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Payload        any         `json:"payload,omitempty"`
	NodeAddress    string      `json:"nodeAddress,omitempty"`
	Status         string      `json:"status,omitempty"`
	EnqueuedAt     time.Time   `json:"enqueuedAt,omitempty"`
	StartedAt      time.Time   `json:"startedAt,omitempty"`
	CompletedAt    time.Time   `json:"completedAt,omitempty"`
	UpdatedAt      time.Time   `json:"updatedAt,omitempty"`
	LastRetryAt    time.Time   `json:"lastRetryAt,omitempty"`
	LastError      string      `json:"lastError,omitempty"`
	ClosedReason   string      `json:"closedReason,omitempty"`
	RetryCount     int         `json:"retryCount,omitempty"`
	FailureHistory []time.Time `json:"failureHistory,omitempty"`
	Persist        bool        `json:"persist,omitempty"`
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
	nodeAddr string
}

func StartEmailDispatcher(ctx context.Context) {
	emailDispatcherOnce.Do(func() {
		queue := make(chan EmailTask, defaultEmailQueueSize)
		done := make(chan struct{})

		nodeAddr, err := GetAddress()
		if err != nil {
			log.Warn("cannot resolve node address for email tasks: %v", err)
		}

		emailDispatcher.mu.Lock()
		emailDispatcher.queue = queue
		emailDispatcher.done = done
		emailDispatcher.stopping = false
		emailDispatcher.nodeAddr = nodeAddr
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

func EnqueueEmailTask(task EmailTask, saveTask bool) {
	if strings.TrimSpace(task.Name) == "" {
		log.Error("email task has empty handler name")
		return
	}
	if _, found := getEmailTaskHandler(task.Name); !found {
		log.Error("email task has unknown handler: %s", task.Name)
		return
	}
	if strings.TrimSpace(task.ID) == "" {
		task.ID = uuid.NewString()
	}

	emailDispatcher.mu.Lock()
	queue := emailDispatcher.queue
	stopping := emailDispatcher.stopping
	nodeAddr := emailDispatcher.nodeAddr
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

	now := time.Now().UTC()
	task.Persist = saveTask
	task.NodeAddress = nodeAddr
	task.Status = emailTaskStatusQueued
	task.EnqueuedAt = now
	task.UpdatedAt = now
	task.StartedAt = time.Time{}
	task.CompletedAt = time.Time{}

	defer func() {
		if rec := recover(); rec != nil {
			log.Warn("email dispatcher queue closed while enqueueing task (%s): %v", task.Name, rec)
		}
	}()
	select {
	case queue <- task:
	default:
		log.Warn("email dispatcher queue full, dropping task: %s", task.Name)
	}
}

func emailDispatcherLoop(queue <-chan EmailTask, done chan struct{}) {
	defer close(done)

	for task := range queue {
		processEmailTask(task)
	}
}

func processEmailTask(task EmailTask) {
	now := time.Now().UTC()
	task.NodeAddress = currentDispatcherNodeAddress()

	if task.Persist {
		task.Status = emailTaskStatusRunning
		task.StartedAt = now
		task.UpdatedAt = now
		saveProceedingTask(task)
	}

	if err := runEmailTask(task); err != nil {
		log.Error("email task failed (%s): %v", task.Name, err)
		if task.Persist {
			failedTask := markTaskFailure(task, err, now)
			clearProceedingTask(failedTask.ID)
			if failedTask.Status == emailTaskStatusFinalFailed {
				clearFailedTask(failedTask.ID)
				saveFinalFailedTask(failedTask)
			} else {
				saveFailedTask(failedTask)
			}
		}
		return
	}

	if task.Persist {
		task.Status = emailTaskStatusSucceeded
		task.CompletedAt = now
		task.UpdatedAt = now
		task.LastError = ""
		task.ClosedReason = ""
		clearProceedingTask(task.ID)
		clearFailedTask(task.ID)
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
	handler, found := getEmailTaskHandler(task.Name)
	if !found {
		return fmt.Errorf("unknown email task handler: %s", task.Name)
	}

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic while processing email task %q: %v", task.Name, rec)
		}
	}()

	return handler(task)
}

const (
	emailTaskStatusQueued      = "queued"
	emailTaskStatusRunning     = "running"
	emailTaskStatusSucceeded   = "succeeded"
	emailTaskStatusFailed      = "failed"
	emailTaskStatusFinalFailed = "final_failed"
)

func RetryErroredEmailTasks() {
	retryStuckProceedingTasks()
	retryFailedTasks()
}

func retryStuckProceedingTasks() {
	tasks, err := loadTasksFromHash(getProceedingTasksHashKey())
	if err != nil {
		log.Warn("cannot load proceeding email tasks from cstore: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, task := range tasks {
		if !task.Persist {
			continue
		}
		if task.Status != emailTaskStatusQueued && task.Status != emailTaskStatusRunning {
			continue
		}
		if !isTaskStuck(task, now) {
			continue
		}

		failureErr := fmt.Errorf("task stuck in proceeding for more than %s", defaultEmailTaskStuckThreshold)
		failedTask := markTaskFailure(task, failureErr, now)
		clearProceedingTask(failedTask.ID)

		if failedTask.Status == emailTaskStatusFinalFailed {
			clearFailedTask(failedTask.ID)
			saveFinalFailedTask(failedTask)
			continue
		}

		saveFailedTask(failedTask)
		EnqueueEmailTask(failedTask, true)
	}
}

func retryFailedTasks() {
	tasks, err := loadTasksFromHash(getFailedTasksHashKey())
	if err != nil {
		log.Warn("cannot load failed email tasks from cstore: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, task := range tasks {
		if !task.Persist || task.Status != emailTaskStatusFailed {
			continue
		}
		if shouldSkipRetryDueToCooldown(task, now) {
			continue
		}

		task.FailureHistory = trimFailureHistory(task.FailureHistory, now)
		task.RetryCount = len(task.FailureHistory)
		if task.RetryCount >= defaultEmailTaskRetryLimit {
			task.Status = emailTaskStatusFinalFailed
			task.ClosedReason = retryLimitExceededReason()
			task.UpdatedAt = now
			task.CompletedAt = now
			clearProceedingTask(task.ID)
			clearFailedTask(task.ID)
			saveFinalFailedTask(task)
			continue
		}

		EnqueueEmailTask(task, true)
	}
}

func shouldSkipRetryDueToCooldown(task EmailTask, now time.Time) bool {
	if task.LastRetryAt.IsZero() {
		return false
	}
	return now.Sub(task.LastRetryAt) < defaultEmailTaskRetryCooldown
}

func isTaskStuck(task EmailTask, now time.Time) bool {
	lastUpdate := task.UpdatedAt
	if lastUpdate.IsZero() {
		lastUpdate = task.EnqueuedAt
	}
	if lastUpdate.IsZero() {
		return false
	}
	return now.Sub(lastUpdate) >= defaultEmailTaskStuckThreshold
}

func markTaskFailure(task EmailTask, runErr error, now time.Time) EmailTask {
	task.LastError = runErr.Error()
	task.UpdatedAt = now
	task.CompletedAt = now
	task.LastRetryAt = now
	task.FailureHistory = append(trimFailureHistory(task.FailureHistory, now), now)
	task.RetryCount = len(task.FailureHistory)

	if task.RetryCount >= defaultEmailTaskRetryLimit {
		task.Status = emailTaskStatusFinalFailed
		task.ClosedReason = retryLimitExceededReason()
		return task
	}

	task.Status = emailTaskStatusFailed
	task.ClosedReason = ""
	return task
}

func retryLimitExceededReason() string {
	return fmt.Sprintf("retry limit reached: %d failures within %s", defaultEmailTaskRetryLimit, defaultEmailTaskRetryWindow)
}

func trimFailureHistory(history []time.Time, now time.Time) []time.Time {
	if len(history) == 0 {
		return nil
	}

	cutoff := now.Add(-defaultEmailTaskRetryWindow)
	kept := make([]time.Time, 0, len(history))
	for _, ts := range history {
		if ts.IsZero() || ts.Before(cutoff) {
			continue
		}
		kept = append(kept, ts)
	}
	return kept
}

func currentDispatcherNodeAddress() string {
	emailDispatcher.mu.Lock()
	defer emailDispatcher.mu.Unlock()

	return emailDispatcher.nodeAddr
}

func isEmailTaskPersistenceEnabled() bool {
	return config.Config.CstoreClient != nil
}

func loadTasksFromHash(hashKey string) ([]EmailTask, error) {
	if !isEmailTaskPersistenceEnabled() || strings.TrimSpace(hashKey) == "" {
		return nil, nil
	}

	client := config.Config.CstoreClient
	items, err := client.HGetAll(context.Background(), hashKey)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	tasks := make([]EmailTask, 0, len(items))
	for _, item := range items {
		var task EmailTask
		if err := json.Unmarshal(item.Value, &task); err != nil {
			log.Warn("cannot decode email task from cstore hash=%s field=%s: %v", hashKey, item.Field, err)
			continue
		}
		if strings.TrimSpace(task.ID) == "" {
			task.ID = item.Field
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func saveProceedingTask(task EmailTask) {
	saveEmailTaskInHash(getProceedingTasksHashKey(), task)
}

func clearProceedingTask(taskID string) {
	clearEmailTaskInHash(getProceedingTasksHashKey(), taskID)
}

func saveFailedTask(task EmailTask) {
	saveEmailTaskInHash(getFailedTasksHashKey(), task)
}

func clearFailedTask(taskID string) {
	clearEmailTaskInHash(getFailedTasksHashKey(), taskID)
}

func saveFinalFailedTask(task EmailTask) {
	task.Status = emailTaskStatusFinalFailed
	saveEmailTaskInHash(getFinalFailedTasksHashKey(), task)
}

func saveEmailTaskInHash(hashKey string, task EmailTask) {
	if !task.Persist || !isEmailTaskPersistenceEnabled() || strings.TrimSpace(hashKey) == "" || strings.TrimSpace(task.ID) == "" {
		return
	}

	if err := config.Config.CstoreClient.HSet(context.Background(), hashKey, task.ID, task, nil); err != nil {
		log.Warn("cannot persist email task state in cstore hash=%s id=%s: %v", hashKey, task.ID, err)
	}
}

func clearEmailTaskInHash(hashKey, taskID string) {
	if !isEmailTaskPersistenceEnabled() || strings.TrimSpace(hashKey) == "" || strings.TrimSpace(taskID) == "" {
		return
	}

	if err := config.Config.CstoreClient.HSet(context.Background(), hashKey, taskID, nil, nil); err != nil {
		log.Warn("cannot clear email task state in cstore hash=%s id=%s: %v", hashKey, taskID, err)
	}
}

func getProceedingTasksHashKey() string {
	if key := strings.TrimSpace(config.Config.CstoreEmailTasksProceedingHashKey); key != "" {
		return key
	}
	return defaultProceedingTasksHashKey
}

func getFailedTasksHashKey() string {
	if key := strings.TrimSpace(config.Config.CstoreEmailTasksFailedHashKey); key != "" {
		return key
	}
	return defaultFailedTasksHashKey
}

func getFinalFailedTasksHashKey() string {
	if key := strings.TrimSpace(config.Config.CstoreEmailTasksFinalFailedHashKey); key != "" {
		return key
	}
	return defaultFinalFailedTasksHashKey
}

func emailTaskErrorFromString(v string) error {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return errors.New(trimmed)
}
