package service

import "testing"


func TestSemaphore(t *testing.T) {
	/*capacity := 2
	sem := NewSem(capacity)

	res := int64(0)
	for i := 0; i < 10; i++ {
		go func() {
			sem.Acquire()

			newRes := atomic.AddInt64(&res, 1)

			require.True(t, newRes <= int64(capacity))

			time.Sleep(1 * time.Second)

			atomic.AddInt64(&res, -1)

			sem.Release()
		}()
	}

	time.Sleep(5 * time.Second)*/
}
