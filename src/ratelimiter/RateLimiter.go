package ratelimiter

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiterInterface interface{
	doGetRate() float64
	doSetRate(permitsPerSecond float64, nowMicros int64)
	queryEarliestAvailable(nowMicros int64) int64
	reserveEarliestAvailable(permits int, nowMicros int64) int64
}


type RateLimiterBase struct{
	mutex sync.Mutex
	stopwatch *SleepingStopwatch

	RateLimiterInterface
}

func maxInt64(x,y int64) int64{
	if x > y {
		return x
	}
	return y
}

func (ratelimiter *RateLimiterBase) SetRate(permitsPerSecond float64) error{
	if err:=checkArgument(permitsPerSecond >0.0, "rate must be positive");err != nil {
		return err
	}
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	ratelimiter.doSetRate(permitsPerSecond,ratelimiter.stopwatch.ReadMicros())
	return nil
}

func (ratelimiter *RateLimiterBase) GetRate() float64{
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	return ratelimiter.doGetRate()
}

func (ratelimiter *RateLimiterBase)reserve(permits int) (int64,error) {
	if err:=ratelimiter.checkPermits(permits) ;err!= nil {
		return -1,err
	}
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()
	return ratelimiter.reserveAndGetWaitLength(permits, ratelimiter.stopwatch.ReadMicros()),nil
}

func (ratelimiter *RateLimiterBase)reserveAndGetWaitLength( permits int, nowMicros int64) int64 {
	momentAvailable := ratelimiter.reserveEarliestAvailable(permits, nowMicros);
	return maxInt64(momentAvailable - nowMicros, 0)
}

func (ratelimiter *RateLimiterBase)checkPermits(permits int) error {
	return checkArgument(permits > 0, fmt.Sprintf("Requested permits (%d) must be positive", permits));
}

func (ratelimiter *RateLimiterBase)Acquire(permits int) (float64,error) {
	microsToWait,err := ratelimiter.reserve(permits)
	if err != nil {
		return -1,err
	}
	ratelimiter.stopwatch.SleepMicrosUninterruptibly(microsToWait);
	return float64(microsToWait*time.Microsecond.Nanoseconds()) / float64(time.Second),nil;
}

func (ratelimiter *RateLimiterBase)canAcquire(nowMicros int64, timeoutMicros int64) bool {
	return (ratelimiter.queryEarliestAvailable(nowMicros) - timeoutMicros) <= nowMicros
}

func (ratelimiter *RateLimiterBase)TryAcquire(permits int, timeout time.Duration)(bool,error) {
	timeoutMicros := maxInt64(timeout.Nanoseconds()/time.Microsecond.Nanoseconds(), 0);
	if err:=ratelimiter.checkPermits(permits);err!=nil{
		return false,err
	}
	var  microsToWait int64
	ratelimiter.mutex.Lock()
	defer ratelimiter.mutex.Unlock()

	nowMicros := ratelimiter.stopwatch.ReadMicros();
	if (!ratelimiter.canAcquire(nowMicros, timeoutMicros)) {
		return false,nil
	} else {
		microsToWait = ratelimiter.reserveAndGetWaitLength(permits, nowMicros);
	}
	ratelimiter.stopwatch.SleepMicrosUninterruptibly(microsToWait);
	return true,nil
}