package ratelimiter

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

type SleepingStopwatch struct{
	Stopwatch
}

func NewSleepingStopwatch()(*SleepingStopwatch,error){
	watch,err:= NewStartedSystemWatch()
	if err!=nil{
		return nil,err
	}
	return &SleepingStopwatch{
		Stopwatch:*watch,
	},nil
}

func (watch *SleepingStopwatch)ReadMicros()int64{
	return watch.getElapsedNanos()/int64(time.Microsecond)
}

func (watch *SleepingStopwatch)SleepMicrosUninterruptibly(micros int64){
	if micros>0{
		time.Sleep(time.Duration(micros*int64(time.Microsecond)))
	}
}

type SmoothWarmingUp struct{
	warmupPeriodMicros int64
	slope float64
	thresholdPermits float64
	coldFactor	float64

	storedPermits float64
	maxPermits float64
	stableIntervalMicros float64
	nextFreeTicketMicros int64

	*RateLimiterBase
}

func checkArgument(b bool,errorMessageTemplate string) error {
	if !b {
		log.Println(errorMessageTemplate)
		return errors.New(errorMessageTemplate)
	}else{
		return nil
	}
}

func NewSmoothWarmingUp(permitsPerSecond float64, warmupPeriod time.Duration, coldFactor float64)(*SmoothWarmingUp,error){
	if err:=checkArgument(warmupPeriod >=0, fmt.Sprintf("warmupPeriod must not be negative: %d",warmupPeriod));err != nil {
		return nil,err
	}
	stopwatch,err :=NewSleepingStopwatch()
	if err != nil {
		return nil,err
	}
	ratelimiter:= SmoothWarmingUp{
		warmupPeriodMicros:warmupPeriod.Nanoseconds()/int64(time.Microsecond),
		coldFactor:coldFactor,
		nextFreeTicketMicros:0,
	}
	ratelimiter.RateLimiterBase=&RateLimiterBase{
		stopwatch:stopwatch,
		RateLimiterInterface:&ratelimiter,
	}
	ratelimiter.SetRate(permitsPerSecond)

	return &ratelimiter,nil
}

func (ratelimiter *SmoothWarmingUp)queryEarliestAvailable(nowMicros int64) int64 {
	return ratelimiter.nextFreeTicketMicros
}
func (ratelimiter *SmoothWarmingUp)doGetRate() float64{
	return float64(time.Second.Nanoseconds()/time.Microsecond.Nanoseconds()) / ratelimiter.stableIntervalMicros;
}

func (ratelimiter *SmoothWarmingUp)resync(nowMicros int64) {
	// if nextFreeTicket is in the past, resync to now
	if (nowMicros > ratelimiter.nextFreeTicketMicros) {
		newPermits := float64(nowMicros - ratelimiter.nextFreeTicketMicros) / ratelimiter.coolDownIntervalMicros();
		ratelimiter.storedPermits = math.Min(ratelimiter.maxPermits, ratelimiter.storedPermits + newPermits);
		ratelimiter.nextFreeTicketMicros = nowMicros;
	}
}

func (ratelimiter *SmoothWarmingUp)doSetRate(permitsPerSecond float64,nowMicros int64) {
	ratelimiter.resync(nowMicros);
	stableIntervalMicros := float64(time.Second/time.Microsecond) / permitsPerSecond;
	ratelimiter.stableIntervalMicros = stableIntervalMicros;

	oldMaxPermits := ratelimiter.maxPermits
	coldIntervalMicros := stableIntervalMicros * ratelimiter.coldFactor
	ratelimiter.thresholdPermits = 0.5 * float64(ratelimiter.warmupPeriodMicros) / stableIntervalMicros
	ratelimiter.maxPermits =
		ratelimiter.thresholdPermits + 2.0 * float64(ratelimiter.warmupPeriodMicros) / (stableIntervalMicros + coldIntervalMicros)
	ratelimiter.slope = (coldIntervalMicros - stableIntervalMicros) / (ratelimiter.maxPermits - ratelimiter.thresholdPermits)
	if math.IsInf(oldMaxPermits,1) {
		ratelimiter.storedPermits = 0.0
	} else {
		if oldMaxPermits == 0.0{
			ratelimiter.storedPermits =ratelimiter.maxPermits
		}else{
			ratelimiter.storedPermits =ratelimiter.storedPermits * ratelimiter.maxPermits / oldMaxPermits
		}
	}
}

func (ratelimiter *SmoothWarmingUp)storedPermitsToWaitTime(storedPermits float64, permitsToTake float64)int64{
	availablePermitsAboveThreshold := storedPermits - ratelimiter.thresholdPermits
	var micros int64 = 0
	if availablePermitsAboveThreshold > 0.0 {
		permitsAboveThresholdToTake := math.Min(availablePermitsAboveThreshold, permitsToTake)
		length :=
			ratelimiter.permitsToTime(availablePermitsAboveThreshold)+ ratelimiter.permitsToTime(availablePermitsAboveThreshold - permitsAboveThresholdToTake)
		micros = int64(permitsAboveThresholdToTake * length / 2.0)
		permitsToTake -= permitsAboveThresholdToTake
	}
	micros += int64(ratelimiter.stableIntervalMicros * permitsToTake)
	return micros
}

func (ratelimiter *SmoothWarmingUp)permitsToTime(permits float64) float64 {
	return ratelimiter.stableIntervalMicros + permits * ratelimiter.slope
}

func (ratelimiter *SmoothWarmingUp)coolDownIntervalMicros() float64 {
	return float64(ratelimiter.warmupPeriodMicros) / ratelimiter.maxPermits
}

func (ratelimiter *SmoothWarmingUp)saturatedAdd(a int64,b int64) int64{
	naiveSum := a + b;
	if (a ^ b) < 0 || (a ^ naiveSum) >= 0 {
		// If a and b have different signs or a has the same sign as the result then there was no
		// overflow, return.
		return naiveSum
	}
	// we did over/under flow, if the sign is negative we should return MAX otherwise MIN
	return math.MaxInt64 + (((naiveSum & (math.MaxInt64>>1)) >> (64 - 1)) ^ 1);
}

func (ratelimiter *SmoothWarmingUp)reserveEarliestAvailable(requiredPermits int, nowMicros int64) int64 {
	ratelimiter.resync(nowMicros);
	returnValue := ratelimiter.nextFreeTicketMicros;
	storedPermitsToSpend := math.Min(float64(requiredPermits), ratelimiter.storedPermits);
	freshPermits := float64(requiredPermits) - storedPermitsToSpend;
	waitMicros :=
		ratelimiter.storedPermitsToWaitTime(ratelimiter.storedPermits, storedPermitsToSpend)+ int64(freshPermits * ratelimiter.stableIntervalMicros);

	ratelimiter.nextFreeTicketMicros = ratelimiter.saturatedAdd(ratelimiter.nextFreeTicketMicros, waitMicros);
	ratelimiter.storedPermits -= storedPermitsToSpend;
	return returnValue
}




