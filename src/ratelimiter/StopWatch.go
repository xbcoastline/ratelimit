package ratelimiter

import (
	"errors"
	"runtime/debug"
	"sync"
	"time"
)

type Ticker interface {
	read() int64
}

type SystemTicker struct {

}

func (ticker *SystemTicker) read()int64{
	return time.Now().UnixNano()
}

type Stopwatch struct {
	ticker Ticker
	isRunning bool
	elapsedNanos int64
	startTick int64
	mutex sync.Mutex
}

func (watch *Stopwatch) getElapsedNanos() int64{
	if watch.isRunning {
		return watch.ticker.read()-watch.startTick+watch.elapsedNanos
	}else{
		return watch.elapsedNanos
	}
}

func (watch *Stopwatch) Start() error{
	watch.mutex.Lock()
	defer watch.mutex.Unlock()
	if watch.isRunning {
		debug.PrintStack()
		return errors.New("This stopwatch is already running.")
	}
	watch.isRunning=true
	watch.startTick = watch.ticker.read()
	return nil
}

func (watch *Stopwatch) Stop() error{
	tick:= watch.ticker.read()
	watch.mutex.Lock()
	defer watch.mutex.Unlock()
	if !watch.isRunning{
		return errors.New("This stopwatch is already stopped.")
	}
	watch.isRunning=false
	watch.elapsedNanos+=tick-watch.startTick
	return nil
}

func (watch *Stopwatch) Reset(){
	watch.isRunning=false
	watch.elapsedNanos=0
	return
}

func (watch *Stopwatch) Elapsed() int64{
	return watch.getElapsedNanos()
}

func NewUnstartedSystemWatch() *Stopwatch{
	return &Stopwatch{
		ticker:  &SystemTicker{},
		isRunning:false,
		elapsedNanos:0,
		startTick:0,
	}
}

func NewStartedSystemWatch() (*Stopwatch,error){
	stopwatch:= Stopwatch{
		ticker:  &SystemTicker{},
		isRunning:false,
		elapsedNanos:0,
		startTick:0,
	}
	return &stopwatch,stopwatch.Start()
}