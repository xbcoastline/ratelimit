package ratelimiter

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiterInterface interface {
	doGetRate() float64
	doSetRate(permitsPerSecond float64, nowTime time.Duration)
	queryEarliestAvailable(nowTime time.Duration) time.Duration
	reserveEarliestAvailable(permits int, nowTime time.Duration) time.Duration
}

type RateLimiterBase struct {
	mutex     sync.Mutex
	stopwatch *SleepingStopwatch

	RateLimiterInterface
}

func maxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func (ratelimiter *RateLimiterBase) SetRate(permitsPerSecond float64) error {
	if err := checkArgument(permitsPerSecond > 0.0, "rate must be positive"); err != nil {
		return err
	}
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	ratelimiter.doSetRate(permitsPerSecond, ratelimiter.stopwatch.Elapsed())
	return nil
}

func (ratelimiter *RateLimiterBase) GetRate() float64 {
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	return ratelimiter.doGetRate()
}

func (ratelimiter *RateLimiterBase) reserve(permits int) (time.Duration, error) {
	if err := ratelimiter.checkPermits(permits); err != nil {
		return -1, err
	}
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	return ratelimiter.reserveAndGetWaitLength(permits, ratelimiter.stopwatch.Elapsed()), nil
}

func (ratelimiter *RateLimiterBase) reserveAndGetWaitLength(permits int, nowTime time.Duration) time.Duration {
	momentAvailable := ratelimiter.reserveEarliestAvailable(permits, nowTime)
	if momentAvailable > nowTime {
		return momentAvailable - nowTime
	} else {
		return 0
	}
}

func (ratelimiter *RateLimiterBase) checkPermits(permits int) error {
	return checkArgument(permits > 0, fmt.Sprintf("Requested permits (%d) must be positive", permits))
}

func (ratelimiter *RateLimiterBase) Acquire(permits int) (float64, error) {
	timeToWait, err := ratelimiter.reserve(permits)
	if err != nil {
		return -1, err
	}
	ratelimiter.stopwatch.ticker.sleep(timeToWait)
	return float64(timeToWait) / float64(time.Second), nil
}

func (ratelimiter *RateLimiterBase) canAcquire(nowTime time.Duration, timeout time.Duration) bool {
	return (ratelimiter.queryEarliestAvailable(nowTime) - timeout) <= nowTime
}

func (ratelimiter *RateLimiterBase) TryAcquire(permits int, timeout time.Duration) (bool, error) {
	if err := ratelimiter.checkPermits(permits); err != nil {
		return false, err
	}
	var timeToWait time.Duration
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()

	nowMicros := ratelimiter.stopwatch.Elapsed()
	if !ratelimiter.canAcquire(nowMicros, timeout) {
		return false, nil
	} else {
		timeToWait = ratelimiter.reserveAndGetWaitLength(permits, nowMicros)
	}
	ratelimiter.stopwatch.ticker.sleep(timeToWait)
	return true, nil
}
