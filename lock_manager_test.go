package redislock

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"
)

func TestLockManagerConcurrency(t *testing.T) {
	t.Run("should acquire a lock", func(t *testing.T) {
		manager, err := NewLockManager("localhost:6379")
		if err != nil {
			t.Error(err)
		}
		defer manager.Close()
		ctx := context.Background()
		lock := manager.Lock(ctx, "some key", 5*time.Second)
		result := lock.Wait()
		if !result.Ok {
			t.Error(result.Error)
		}
		err = lock.Unlock()
		if err != nil {
			t.Error(err)
			t.Fail()
		}
		if _, ok := lock.manager.queue["some key"]; ok {
			t.Errorf("Did not clear up the queue")
		}
	})

	t.Run("two threads one holds one waits", func(t *testing.T) {
		manager, err := NewLockManager("localhost:6379")
		if err != nil {
			log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer manager.Close()
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done() // Decrements the counter by one when the goroutine completes
			ctx := context.Background()
			lock := manager.Lock(ctx, "key", 5*time.Second)
			if err != nil {
				t.Error(err)
			}
			result := lock.Wait()
			if !result.Ok {
				t.Errorf("not ok %+s", result.Error)
			}
			// this thread will hold on to the
			time.Sleep(2 * time.Second)
			err = lock.Unlock()
			if err != nil {
				t.Error(err)
			}
		}()

		go func() {
			defer wg.Done() // Decrements the counter by one when the goroutine completes
			ctx := context.Background()
			lock := manager.Lock(ctx, "key", 5*time.Second)
			result := lock.Wait()
			if !result.Ok {
				t.Errorf("not ok %+s", result.Error)
			}
			err = lock.Unlock()
			if err != nil {
				t.Error(err)
			}
		}()

		wg.Wait()
	})

	t.Run("two threads one holds until timeout", func(t *testing.T) {
		manager, err := NewLockManager("localhost:6379")
		if err != nil {
			log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer manager.Close()
		var wg sync.WaitGroup
		wg.Add(2)
		check := []bool{false, false}

		go func() {
			defer func() {
				wg.Done()
			}()

			ctx := context.Background()
			lock := manager.Lock(ctx, "key", 1*time.Second)
			result := lock.Wait()
			if !result.Ok {
				t.Errorf("not ok %+s", result.Error)
			}
			time.Sleep(2 * time.Second)
			lock.Unlock()
			check[0] = true
		}()

		go func() {
			defer func() {
				wg.Done() // Decrements the counter by one when the goroutine completes
			}()

			ctx := context.Background()
			lock := manager.Lock(ctx, "key", 1*time.Second)
			if err != nil {
				t.Error(err)
			}
			result := lock.Wait()
			if !result.Ok {
				t.Errorf("not ok %+s", result.Error)
			}
			lock.Unlock()
			check[1] = true
		}()

		wg.Wait()

		for _, c := range check {
			if !c {
				t.Error("check failed")
			}
		}
	})

	t.Run("One hundre threads on same key", func(t *testing.T) {
		option := DefaultOption
		option.MaxActive = 500
		option.MaxQueueSize = 10

		manager, err := NewLockManager("localhost:6379", option)
		if err != nil {
			log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer manager.Close()
		var wg sync.WaitGroup
		threadCount := 100
		wg.Add(threadCount)
		check := [100]bool{}
		for i := 0; i < threadCount; i++ {
			idx := i
			go func() {
				defer func() {
					wg.Done()
				}()
				ctx := context.Background()
				lock := manager.Lock(ctx, "key", 1*time.Second)
				result := lock.Wait()
				if !result.Ok {
					t.Errorf("not ok %s %s", result.Error, lock.value)
				}
				check[idx] = true
				lock.Unlock()
			}()
		}
		wg.Wait()
		for _, c := range check {
			if !c {
				t.Error("check failed")
			}
		}
		if len(manager.(*lockManager).queue) != 0 {
			t.Error("queue should be empty")
		}
	})

	t.Run("One thousand threads on differnt key", func(t *testing.T) {
		option := DefaultOption
		option.MaxActive = 500
		option.MaxQueueSize = 10
		manager, err := NewLockManager("localhost:6379", option)
		if err != nil {
			log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer manager.Close()
		var wg sync.WaitGroup
		threadCount := 1000
		wg.Add(threadCount)
		check := [1000]bool{}
		for i := 0; i < threadCount; i++ {
			time.Sleep(50 * time.Microsecond)
			idx := i
			go func() {
				defer func() {
					wg.Done()
				}()
				ctx := context.Background()
				lock := manager.Lock(ctx, generateUniqueID(), 1*time.Second)
				result := lock.Wait()
				if !result.Ok {
					t.Errorf("not ok %s %s", result.Error, lock.value)
				}
				check[idx] = true
				lock.Unlock()
			}()
		}
		wg.Wait()
		for _, c := range check {
			if !c {
				t.Error("check failed")
			}
		}
		if len(manager.(*lockManager).queue) != 0 {
			t.Error("queue should be empty")
		}
	})
}
