package ratelimiter

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type SlidingWindowLimiter struct {
	prevSlotCount	int64
	prevSlotSec		int64
	curSlotCount	int64
	curSlotSec		int64
	permitPerSec	int64

	mux				sync.RWMutex
}

func NewSlidingLimiter(permitsPerSecond int64)(*SlidingWindowLimiter,error){
	//max ratelimit at one minute
	if permitsPerSecond <1 {
		return nil,errors.New("permitsPerSecond must >1")
	}
	curSec:=time.Now().Unix()
	limiter:=&SlidingWindowLimiter{
		prevSlotCount:0,
		prevSlotSec:curSec-1,
		curSlotCount:0,
		curSlotSec:curSec,
		permitPerSec:permitsPerSecond,
	}
	return limiter,nil
}

func (rl *SlidingWindowLimiter)shiftSlot(newTime time.Time) {
	newSec:=newTime.Unix()
	rl.mux.Lock()
	if rl.curSlotSec < newSec{
		if rl.curSlotSec == newSec-1{
			rl.prevSlotCount=rl.curSlotCount
		}else{
			rl.prevSlotCount=0
		}
		rl.prevSlotSec=newSec-1
		rl.curSlotSec=newSec
		rl.curSlotCount=0
	}
	rl.mux.Unlock()
}

func getSlideSlotCount(newTime time.Time,prevCount int64,curCount int64) int64 {
	crossPercent:=1-float64(newTime.Nanosecond())/float64(time.Second.Nanoseconds())
	thresholdValue:=int64(crossPercent*float64(prevCount))+curCount

	return thresholdValue
}

//max can acqui
func (rl *SlidingWindowLimiter)Acquire(permits int) (error) {
	newTime:=time.Now()
	newSec:=newTime.Unix()
	if rl.curSlotSec < newSec{
		rl.shiftSlot(newTime)
	}
	rl.mux.RLock()
	defer rl.mux.RUnlock()
	thresholdValue:=getSlideSlotCount(newTime,rl.prevSlotCount,rl.curSlotCount)
	if rl.permitPerSec > thresholdValue {
		if getSlideSlotCount(newTime,rl.prevSlotCount,atomic.AddInt64(&rl.curSlotCount,1)) > rl.permitPerSec{
			atomic.AddInt64(&rl.curSlotCount,-1)
			return errors.New("rate limit 1")
		}else{
			return nil
		}
	}else{
		return errors.New("rate limit 2")
	}
	atomic.AddInt64(&rl.curSlotCount,1)

	return nil
}
