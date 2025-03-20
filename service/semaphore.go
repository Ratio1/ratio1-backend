package service

type Semaphore struct {
	semC chan struct{}
}

func NewSem(capacity int) *Semaphore {
	return &Semaphore{
		semC: make(chan struct{}, capacity),
	}
}

func (s *Semaphore) Acquire() {
	s.semC <- struct{}{}
}

func (s *Semaphore) Release() {
	<-s.semC
}
