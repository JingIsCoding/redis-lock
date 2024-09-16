package redislock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Lock struct {
	manager    *lockManager
	key        string
	value      string
	resultChan chan result
}

func (lock *Lock) Unlock() error {
	return lock.manager.releaseLock(lock.key, lock.value)
}

func (lock *Lock) Wait() result {
	return <-lock.resultChan
}

type result struct {
	Ok    bool
	Error error
}

type LockManager interface {
	Lock(context context.Context, key string, duration time.Duration) Lock
	Close() error
}

type lockRequest struct {
	key        string
	value      string
	redisChan  chan string
	resultChan chan result
	logger     Logger
}

type lockManager struct {
	pool         *redis.Pool
	pubsub       *redis.PubSubConn
	queue        map[string][]lockRequest
	mu           sync.Mutex
	logger       Logger
	maxQueueSize int
}

func (manager *lockManager) Lock(c context.Context, key string, ttl time.Duration) Lock {
	resultChan := make(chan result, 1)
	redisChan := make(chan string, 1)
	value := generateUniqueID()
	go func() {
		ctx, cancel := context.WithTimeout(c, ttl+time.Second)
		defer func() {
			if r := recover(); r != nil {
				manager.logger.Error("Recovered from panic:", r)
			}
			cancel()
			manager.removeRequest(key, value)
			close(redisChan)
			close(resultChan)
		}()

		request := lockRequest{
			key:        key,
			value:      value,
			redisChan:  redisChan,
			resultChan: resultChan,
		}

		// If there no other request waiting for this key
		if size := manager.queueSize(key); size == 0 {
			ok, err := manager.acquireLock(key, value, ttl)
			if err != nil {
				select {
				case resultChan <- result{Ok: ok, Error: err}:
					return
				default:
				}
				return
			}
			if ok {
				select {
				case resultChan <- result{Ok: ok}:
					// if we acquire the lock successfully, return immediately
					return
				default:
				}
				return
			}
			// otherwise the request will be added to queue
		} else if size > manager.maxQueueSize {
			resultChan <- result{Ok: false, Error: errors.New("Too many requests ")}
			return
		}

		manager.addToQueue(key, request)

		for {
			select {
			// if thread cancel the context
			case <-ctx.Done():
				return
			// if redis tells us the key is released or expired
			case <-redisChan:
				ok, err := manager.acquireLock(key, value, ttl)
				if ok || err != nil {
					select {
					case resultChan <- result{Ok: ok, Error: err}:
						return
					default:
						continue
					}
				}
			}
		}
	}()

	return Lock{
		manager:    manager,
		key:        key,
		value:      value,
		resultChan: resultChan,
	}
}

func (manager *lockManager) queueSize(key string) int {
	requests, ok := manager.queue[key]
	if ok {
		return len(requests)
	}
	return 0
}

func (manager *lockManager) addToQueue(key string, requst lockRequest) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if requests, ok := manager.queue[key]; ok {
		manager.queue[key] = append(requests, requst)
	} else {
		manager.queue[key] = []lockRequest{requst}
	}
}

func (manager *lockManager) removeRequest(key string, value string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if requests, ok := manager.queue[key]; ok {
		for i, req := range requests {
			if req.value == value {
				requests = append(requests[:i], requests[i+1:]...)
				if len(requests) == 0 {
					delete(manager.queue, key)
				} else {
					manager.queue[key] = requests
				}

				break
			}
		}
	}
}

func (l *lockManager) Close() error {
	for _, requests := range l.queue {
		for _, req := range requests {
			close(req.resultChan)
			close(req.redisChan)
		}
	}

	if err := l.pubsub.PUnsubscribe(); err != nil {
		l.logger.Error("", err)
		return err
	}

	if err := l.pool.Close(); err != nil {
		l.logger.Error("", err)
		return err
	}

	return nil
}

func (l *lockManager) listenToKeyChange() error {
	if err := l.pubsub.PSubscribe("__keyevent@0__:expired", "__keyevent@0__:del"); err != nil {
		return err
	}
	for {
		switch v := l.pubsub.Receive().(type) {
		case redis.Message:
			key := string(v.Data)
			if requests, ok := l.queue[key]; ok {
				if len(requests) > 0 {
					// TODO, fill the logic to find the first valid request to send
					for _, req := range requests {
						select {
						case req.redisChan <- "ready":
							break
						default:
							// This channel is either full or closed, try the next one
						}
					}
				}
			}
		case redis.Subscription:
			l.logger.Info("Subscription event %s %s\n", v.Kind, v.Channel)
			continue
		case error:
			return v
		default:
			l.logger.Info("Unknown message")
		}
	}
}

func (l *lockManager) acquireLock(key string, value string, ttl time.Duration) (bool, error) {
	conn := l.pool.Get()
	defer conn.Close()
	reply, err := redis.String(conn.Do("SET", key, value, "NX", "PX", int(ttl.Milliseconds())))
	if err != nil {
		if err == redis.ErrNil {
			return false, nil
		}
		return false, err
	}
	return reply == "OK", nil
}

func (l *lockManager) releaseLock(key string, value string) error {
	conn := l.pool.Get()
	defer conn.Close()

	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
	result, err := redis.Int(conn.Do("EVAL", script, 1, key, value))
	if err != nil {
		return err
	}
	if result != 1 {
		return errors.New("wrong value")
	}
	return nil
}

func generateUniqueID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

type Option struct {
	MaxIdle      int
	MaxActive    int
	MaxQueueSize int
	IdleTimeout  time.Duration
	Logger       Logger
}

var DefaultOption = Option{
	MaxIdle:      5,
	MaxActive:    100,
	MaxQueueSize: 10,
	IdleTimeout:  240 * time.Second,
	Logger:       Default,
}

func NewLockManager(redisAddr string, optionalOption ...Option) (LockManager, error) {
	var option Option
	if len(optionalOption) == 0 {
		option = DefaultOption
	} else {
		option = optionalOption[0]
	}
	pool := &redis.Pool{
		MaxIdle:     option.MaxIdle,
		MaxActive:   option.MaxActive,
		IdleTimeout: option.IdleTimeout,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisAddr)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	manager := &lockManager{
		pool:         pool,
		queue:        make(map[string][]lockRequest),
		logger:       option.Logger,
		maxQueueSize: option.MaxQueueSize,
	}
	if _, err := pool.Get().Do("CONFIG", "SET", "notify-keyspace-events", "AKE"); err != nil {
		return manager, err
	}
	pubsubConn := pool.Get()
	manager.pubsub = &redis.PubSubConn{Conn: pubsubConn}
	go func() error {
		defer func() {
			if r := recover(); r != nil {
				manager.logger.Error("Recovered from panic:", r)
			}
		}()
		return manager.listenToKeyChange()
	}()
	return manager, nil
}
