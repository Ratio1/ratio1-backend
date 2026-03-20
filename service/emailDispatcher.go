package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/google/uuid"
)

/*
..######..########.########..##.....##..######..########
.##....##....##....##.....##.##.....##.##....##....##...
.##..........##....##.....##.##.....##.##..........##...
..######.....##....########..##.....##.##..........##...
.......##....##....##...##...##.....##.##..........##...
.##....##....##....##....##..##.....##.##....##....##...
..######.....##....##.....##..#######...######.....##...
*/

type EmailTask struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Payload        any         `json:"payload,omitempty"`
	NodeAddress    string      `json:"nodeAddress,omitempty"`
	EnqueuedAt     time.Time   `json:"enqueuedAt,omitempty"`
	UpdatedAt      time.Time   `json:"updatedAt,omitempty"`
	LastRetryAt    time.Time   `json:"lastRetryAt,omitempty"`
	Errors         []string    `json:"lastError,omitempty"`
	RetryCount     int         `json:"retryCount,omitempty"`
	FailureHistory []time.Time `json:"failureHistory,omitempty"`
	Persist        bool        `json:"persist,omitempty"`
}

type dispatcherState struct {
	mu       sync.Mutex
	queue    chan EmailTask
	done     chan struct{}
	stopping bool
	nodeAddr string
	ctx      context.Context
}

/*
.##.....##....###....########...######.
.##.....##...##.##...##.....##.##....##
.##.....##..##...##..##.....##.##......
.##.....##.##.....##.########...######.
..##...##..#########.##...##.........##
...##.##...##.....##.##....##..##....##
....###....##.....##.##.....##..######.
*/
var (
	emailDispatcherOnce sync.Once
	emailDispatcher     dispatcherState
)

const defaultEmailQueueSize = 10000
const defaultEmailTaskRetryLimit = 3
const defaultEmailTaskStuckThreshold = 3 * time.Hour
const defaultEmailTaskRetryCooldown = 5 * time.Minute
const defaultProceedingTasksHashKey = "ratio1_email_tasks_proceeding"
const defaultFailedTasksHashKey = "ratio1_email_tasks_failed"
const defaultFinalFailedTasksHashKey = "ratio1_email_tasks_final_failed"

/*
.##.....##....###....####.##....##....########.##.....##.##....##..######...######.
.###...###...##.##....##..###...##....##.......##.....##.###...##.##....##.##....##
.####.####..##...##...##..####..##....##.......##.....##.####..##.##.......##......
.##.###.##.##.....##..##..##.##.##....######...##.....##.##.##.##.##........######.
.##.....##.#########..##..##..####....##.......##.....##.##..####.##.............##
.##.....##.##.....##..##..##...###....##.......##.....##.##...###.##....##.##....##
.##.....##.##.....##.####.##....##....##........#######..##....##..######...######.
*/

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
		emailDispatcher.ctx = ctx
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

func EnqueueEmailTask(task EmailTask, saveTask bool) { //TO Be called outside of the process
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
		log.Error("email dispatcher not started, dropping task: %s", task.Name)
		return
	}
	if stopping {
		emailDispatcher.mu.Unlock()
		log.Warn("email dispatcher is stopping, dropping task: %s", task.Name)
		return
	}
	emailDispatcher.mu.Unlock()

	now := time.Now().UTC()
	task.ID = uuid.NewString()
	task.Persist = saveTask
	task.NodeAddress = nodeAddr
	task.EnqueuedAt = now
	task.UpdatedAt = now
	task.RetryCount = defaultEmailTaskRetryLimit

	//saving in proceeding before actually running the task ( so that if it fails while in queue it doesn't get lost)
	if task.Persist {
		err := saveOrUpdateInCstore(emailDispatcher.ctx, getProceedingTasksHashKey(), task.ID, task)
		if err != nil {
			log.Error("error on saving email in proceeding task: %s", task.Name)
		}
	}

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

func RetryErroredEmailTasks() {
	retryStuckProceedingTasks()
	retryFailedTasks()
}

/*
.########..########...#######...######..########..######...######.....########.##.....##.##....##..######.
.##.....##.##.....##.##.....##.##....##.##.......##....##.##....##....##.......##.....##.###...##.##....##
.##.....##.##.....##.##.....##.##.......##.......##.......##..........##.......##.....##.####..##.##......
.########..########..##.....##.##.......######....######...######.....######...##.....##.##.##.##.##......
.##........##...##...##.....##.##.......##.............##.......##....##.......##.....##.##..####.##......
.##........##....##..##.....##.##....##.##.......##....##.##....##....##.......##.....##.##...###.##....##
.##........##.....##..#######...######..########..######...######.....##........#######..##....##..######.
*/

func emailDispatcherLoop(queue <-chan EmailTask, done chan struct{}) {
	defer close(done)

	for task := range queue {
		processEmailTask(task)
	}
}

func processEmailTask(task EmailTask) {
	now := time.Now().UTC()
	emailDispatcher.mu.Lock()
	task.NodeAddress = emailDispatcher.nodeAddr
	ctx := emailDispatcher.ctx
	emailDispatcher.mu.Unlock()

	if err := runEmailTask(task); err != nil {
		if task.Persist {
			task.UpdatedAt = now
			task.Errors = append(task.Errors, err.Error())
			task.FailureHistory = append(task.FailureHistory, now)
			saveOrUpdateInCstore(ctx, getProceedingTasksHashKey(), task.ID, nil) //remove from proceeding
			task.RetryCount = task.RetryCount - 1
			if task.RetryCount == 0 {
				saveOrUpdateInCstore(ctx, getFinalFailedTasksHashKey(), task.ID, task)
			} else {
				saveOrUpdateInCstore(ctx, getFailedTasksHashKey(), task.ID, task)
			}
		}
		return
	}

	saveOrUpdateInCstore(ctx, getProceedingTasksHashKey(), task.ID, nil) //remove from proceeding
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

func retryStuckProceedingTasks() {
	emailDispatcher.mu.Lock()
	ctx := emailDispatcher.ctx
	emailDispatcher.mu.Unlock()
	tasks, err := fetchAllFromCstore(ctx, getProceedingTasksHashKey())
	if err != nil {
		log.Warn("cannot load proceeding email tasks from cstore: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, task := range tasks {
		if !task.Persist {
			continue
		}
		if !isTaskStuck(task, now) {
			continue
		}

		failureErr := fmt.Errorf("task stuck in proceeding for more than %s", defaultEmailTaskStuckThreshold)
		task.RetryCount -= 1
		task.Errors = append(task.Errors, failureErr.Error())
		task.FailureHistory = append(task.FailureHistory, now)
		task.LastRetryAt = now
		task.UpdatedAt = now
		task.NodeAddress = emailDispatcher.nodeAddr
		if task.RetryCount == 0 {
			saveOrUpdateInCstore(ctx, getProceedingTasksHashKey(), task.ID, nil)   //remove from proceeding
			saveOrUpdateInCstore(ctx, getFinalFailedTasksHashKey(), task.ID, task) //add to final failed
		} else {
			saveOrUpdateInCstore(ctx, getProceedingTasksHashKey(), task.ID, task) //update to prevent other nodes to fetch stuck task
			enqueueStuckAndFailedTasks(task)
		}
	}
}

func retryFailedTasks() {
	emailDispatcher.mu.Lock()
	ctx := emailDispatcher.ctx
	emailDispatcher.mu.Unlock()
	tasks, err := fetchAllFromCstore(ctx, getFailedTasksHashKey())
	if err != nil {
		log.Warn("cannot load proceeding email tasks from cstore: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, task := range tasks {
		if !task.Persist {
			continue
		}
		if shouldSkipRetryDueToCooldown(task, now) {
			continue
		}
		task.LastRetryAt = now
		task.UpdatedAt = now
		task.NodeAddress = emailDispatcher.nodeAddr
		saveOrUpdateInCstore(ctx, getProceedingTasksHashKey(), task.ID, task) //add task to processing
		saveOrUpdateInCstore(ctx, getFailedTasksHashKey(), task.ID, nil)      //remove from failed tasks
		enqueueStuckAndFailedTasks(task)
	}
}

func enqueueStuckAndFailedTasks(task EmailTask) {
	emailDispatcher.mu.Lock()
	queue := emailDispatcher.queue
	stopping := emailDispatcher.stopping

	if queue == nil {
		emailDispatcher.mu.Unlock()
		if stopping {
			log.Warn("email dispatcher is stopping, dropping task: %s", task.Name)
			return
		}
		log.Error("email dispatcher not started, dropping task: %s", task.Name)
		return
	}
	if stopping {
		emailDispatcher.mu.Unlock()
		log.Warn("email dispatcher is stopping, dropping task: %s", task.Name)
		return
	}
	emailDispatcher.mu.Unlock()

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

/*
.##.....##.########.####.##........######.
.##.....##....##.....##..##.......##....##
.##.....##....##.....##..##.......##......
.##.....##....##.....##..##........######.
.##.....##....##.....##..##.............##
.##.....##....##.....##..##.......##....##
..#######.....##....####.########..######.
*/

func fetchAllFromCstore(ctx context.Context, hashKey string) ([]EmailTask, error) {
	items, err := config.Config.CstoreClient.HGetAll(ctx, hashKey)
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

func saveOrUpdateInCstore(ctx context.Context, hashKey, key string, value any) error {
	return config.Config.CstoreClient.HSet(ctx, hashKey, key, value, nil)
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

func shouldSkipRetryDueToCooldown(task EmailTask, now time.Time) bool {
	if task.LastRetryAt.IsZero() {
		return false
	}
	return now.Sub(task.LastRetryAt) < defaultEmailTaskRetryCooldown
}
