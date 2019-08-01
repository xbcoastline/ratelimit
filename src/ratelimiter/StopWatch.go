package ratelimiter

import (
	"errors"
	"runtime/debug"
	"sync"
	"time"
)

type Ticker interface {
	read() int64
	sleep(duration time.Duration)
}

type SystemTicker struct {
}

func (ticker *SystemTicker) read() int64 {
	return time.Now().UnixNano()
}

func (ticker *SystemTicker) sleep(duration time.Duration) {
	time.Sleep(duration)
}

type Stopwatch struct {
	ticker      Ticker
	isRunning   bool
	elapsedTime time.Duration
	startTime   time.Duration
	mutex       sync.Mutex
}

func (watch *Stopwatch) getElapsedTime() time.Duration {
	if watch.isRunning {
		return time.Duration(watch.ticker.read()) - watch.startTime + watch.elapsedTime
	} else {
		return watch.elapsedTime
	}
}

func (watch *Stopwatch) Start() error {
	watch.mutex.Lock()
	defer watch.mutex.Unlock()
	if watch.isRunning {
		debug.PrintStack()
		return errors.New("This stopwatch is already running.")
	}
	watch.isRunning = true
	watch.startTime = time.Duration(watch.ticker.read())
	return nil
}

func (watch *Stopwatch) Stop() error {
	tick := watch.ticker.read()
	watch.mutex.Lock()
	defer watch.mutex.Unlock()
	if !watch.isRunning {
		return errors.New("This stopwatch is already stopped.")
	}
	watch.isRunning = false
	watch.elapsedTime += time.Duration(tick) - watch.startTime
	return nil
}

func (watch *Stopwatch) Reset() {
	watch.isRunning = false
	watch.elapsedTime = 0
	return
}

func (watch *Stopwatch) Elapsed() time.Duration {
	return watch.getElapsedTime()
}

func NewUnstartedSystemWatch() *Stopwatch {
	return &Stopwatch{
		ticker:      &SystemTicker{},
		isRunning:   false,
		elapsedTime: 0,
		startTime:   0,
	}
}

func NewStartedSystemWatch() (*Stopwatch, error) {
	stopwatch := Stopwatch{
		ticker:      &SystemTicker{},
		isRunning:   false,
		elapsedTime: 0,
		startTime:   0,
	}
	return &stopwatch, stopwatch.Start()
}
