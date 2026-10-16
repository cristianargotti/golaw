package jobs

import "sync"

// Start runs the worker in a goroutine.
func Start(work func()) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		work()
	}()
	return &wg
}
