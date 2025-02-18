package process

import (
	"time"
)

type Timer struct {
	timer *time.Timer
}

func NewTimer(jobTimestamp int64, job func()) (*Timer, error) {
	now := time.Now().Unix()
	delaySeconds := now // initialize with a big value

	if now < jobTimestamp {
		delaySeconds = jobTimestamp - now
	}

	// Schedule the task with the calculated delay
	delayDuration := time.Duration(delaySeconds) * time.Second
	return &Timer{timer: time.AfterFunc(delayDuration, job)}, nil
}

func (t *Timer) Start() {
	//nothing to do
}

func (t *Timer) Stop() {
	t.timer.Stop()
}
