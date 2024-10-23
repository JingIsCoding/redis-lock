## redis-lock

#### Context
redis-lock is a Golang library designed to provide a reliable and efficient way for threads to acquire and release locks using Redis. This library offers a simple interface for thread-safe distributed locking, leveraging Redis Pub/Sub to minimize blocking connections and reduce overhead from interval polling.

#### Features 
- Define a standard interface for acquiring and releasing locks.
- Handle waiting for locks to be released or expired if currently held by other threads.
- Use Redis Pub/Sub to avoid interval-based polling and minimize unnecessary load on Redis.
- Easily integrate with existing projects using a minimalistic API.

#### Installation 
``` bash
go get -u github.com/JingIsCoding/redis-lock
```

#### Usage 

##### Acquire a lock
``` golang
package main

import (
	"context"
	"log"
	"time"

	redislock "github.com/JingIsCoding/redis-lock"
)

func main() {
	manager, err := redislock.NewLockManager("localhost:6379")
	if err != nil {
		log.Panicln(err)
	}
	defer manager.Close()
	ctx := context.Background()
	lock := manager.Lock(ctx, "some key", 5*time.Second)
	result := lock.Wait()
	if !result.Ok {
		log.Panicln(result.Error)
	}
	err = lock.Unlock()
	if err != nil {
		log.Panicln(result.Error)
	}

}

```

##### Custom Options
``` golang
package main

import (
	"context"
	"log"
	"time"

	redislock "github.com/JingIsCoding/redis-lock"
)

type CustomLogger struct{}

func (c *CustomLogger) LogMode(_ redislock.LogLevel) redislock.Logger {
	panic("not implemented") // TODO: Implement
}

func (c *CustomLogger) Info(_ string, _ ...interface{}) {
	panic("not implemented") // TODO: Implement
}

func (c *CustomLogger) Warn(_ string, _ ...interface{}) {
	panic("not implemented") // TODO: Implement
}

func (c *CustomLogger) Error(_ string, _ ...interface{}) {
	panic("not implemented") // TODO: Implement
}

func main() {
	option := redislock.DefaultOption
    	option.MaxActive = 500
	option.MaxQueueSize = 10
	option.Logger = &CustomLogger{}

	manager, err := redislock.NewLockManager("localhost:6379", option)
	if err != nil {
		log.Panicln(err)
	}
	defer manager.Close()
	ctx := context.Background()
	lock := manager.Lock(ctx, "some key", 5*time.Second)
	result := lock.Wait()
	if !result.Ok {
		log.Panicln(result.Error)
	}
	err = lock.Unlock()
	if err != nil {
		log.Panicln(result.Error)
	}

}

```

#### API Reference
##### Lock manager
`Lock(ctx context.Context, key string, ttl time.Duration) Lock ` \
Immediately get a Lock object to wait on or unlock

##### Lock
`Unlock() error` \
Releases the lock held by on the key with this `Lock`

`Wait() result` \
Wait on the lock when the key is either acquired immediately or notified when the key is released or expired


#### Redis Pub/Sub Optimization
The library utilizes Redis Pub/Sub to wait for lock release events instead of using interval polling. This approach significantly reduces the number of Redis queries and avoids excessive load, making it suitable for high-throughput distributed systems.
