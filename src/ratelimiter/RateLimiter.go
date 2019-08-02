package ratelimiter

import (
	"fmt"
	"sync"
	"time"
)

type LimiterInterface interface {
	doGetRate() float64
	doSetRate(permitsPerSecond float64, nowTime time.Duration)
	queryEarliestAvailable(nowTime time.Duration) time.Duration
	reserveEarliestAvailable(permits int, nowTime time.Duration) time.Duration
}

type BaseLimiter struct {
	mutex     sync.Mutex
	stopwatch *Stopwatch

	LimiterInterface
}

func (limiter *BaseLimiter) SetRate(permitsPerSecond float64) error {
	if err := checkArgument(permitsPerSecond > 0.0, "rate must be positive"); err != nil {
		return err
	}
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	limiter.doSetRate(permitsPerSecond, limiter.stopwatch.Elapsed())
	return nil
}

func (limiter *BaseLimiter) GetRate() float64 {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	return limiter.doGetRate()
}

func (limiter *BaseLimiter) reserve(permits int) (time.Duration, error) {
	if err := limiter.checkPermits(permits); err != nil {
		return -1, err
	}
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	return limiter.reserveAndGetWaitLength(permits, limiter.stopwatch.Elapsed()), nil
}

func (limiter *BaseLimiter) reserveAndGetWaitLength(permits int, nowTime time.Duration) time.Duration {
	momentAvailable := limiter.reserveEarliestAvailable(permits, nowTime)
	if momentAvailable > nowTime {
		return momentAvailable - nowTime
	} else {
		return 0
	}
}

func (limiter *BaseLimiter) checkPermits(permits int) error {
	return checkArgument(permits > 0, fmt.Sprintf("Requested permits (%d) must be positive", permits))
}

func (limiter *BaseLimiter) Acquire(permits int) (float64, error) {
	timeToWait, err := limiter.reserve(permits)
	if err != nil {
		return -1, err
	}
	limiter.stopwatch.ticker.sleep(timeToWait)
	return float64(timeToWait) / float64(time.Second), nil
}

func (limiter *BaseLimiter) canAcquire(nowTime time.Duration, timeout time.Duration) bool {
	return (limiter.queryEarliestAvailable(nowTime) - timeout) <= nowTime
}

func (limiter *BaseLimiter) TryAcquire(permits int, timeout time.Duration) (bool, error) {
	if err := limiter.checkPermits(permits); err != nil {
		return false, err
	}
	var timeToWait time.Duration
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	nowMicros := limiter.stopwatch.Elapsed()
	if !limiter.canAcquire(nowMicros, timeout) {
		return false, nil
	} else {
		timeToWait = limiter.reserveAndGetWaitLength(permits, nowMicros)
	}
	limiter.stopwatch.ticker.sleep(timeToWait)
	return true, nil
}
