package ratelimiter

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

type SleepingStopwatch struct {
	Stopwatch
}

func NewSleepingStopwatch() (*SleepingStopwatch, error) {
	watch, err := NewStartedSystemWatch()
	if err != nil {
		return nil, err
	}
	return &SleepingStopwatch{
		Stopwatch: *watch,
	}, nil
}

type SmoothWarmingUp struct {
	warmupPeriodTime time.Duration
	slope            float64
	thresholdPermits float64
	coldFactor       float64

	storedPermits      float64
	maxPermits         float64
	stableIntervalTime time.Duration
	nextFreeTicketTime time.Duration

	*RateLimiterBase
}

func checkArgument(b bool, errorMessageTemplate string) error {
	if !b {
		log.Println(errorMessageTemplate)
		return errors.New(errorMessageTemplate)
	} else {
		return nil
	}
}

func NewSmoothWarmingUp(permitsPerSecond float64, warmupPeriod time.Duration, coldFactor float64) (*SmoothWarmingUp, error) {
	if err := checkArgument(warmupPeriod >= 0, fmt.Sprintf("warmupPeriod must not be negative: %d", warmupPeriod)); err != nil {
		return nil, err
	}
	stopwatch, err := NewSleepingStopwatch()
	if err != nil {
		return nil, err
	}
	ratelimiter := SmoothWarmingUp{
		warmupPeriodTime:   warmupPeriod,
		coldFactor:         coldFactor,
		nextFreeTicketTime: 0,
	}
	ratelimiter.RateLimiterBase = &RateLimiterBase{
		stopwatch:            stopwatch,
		RateLimiterInterface: &ratelimiter,
	}
	if err := ratelimiter.SetRate(permitsPerSecond); err != nil {
		return nil, err
	}

	return &ratelimiter, nil
}

func (ratelimiter *SmoothWarmingUp) queryEarliestAvailable(nowTime time.Duration) time.Duration {
	return ratelimiter.nextFreeTicketTime
}
func (ratelimiter *SmoothWarmingUp) doGetRate() float64 {
	return float64(time.Second) / float64(ratelimiter.stableIntervalTime)
}

func (ratelimiter *SmoothWarmingUp) resync(nowTime time.Duration) {
	// if nextFreeTicket is in the past, resync to now
	if nowTime > ratelimiter.nextFreeTicketTime {
		newPermits := float64(nowTime-ratelimiter.nextFreeTicketTime) / ratelimiter.coolDownIntervalTime()
		ratelimiter.storedPermits = math.Min(ratelimiter.maxPermits, ratelimiter.storedPermits+newPermits)
		ratelimiter.nextFreeTicketTime = nowTime
	}
}

func (ratelimiter *SmoothWarmingUp) doSetRate(permitsPerSecond float64, nowTime time.Duration) {
	ratelimiter.resync(nowTime)
	stableIntervalTime := time.Duration(float64(time.Second) / permitsPerSecond)
	ratelimiter.stableIntervalTime = stableIntervalTime

	oldMaxPermits := ratelimiter.maxPermits
	coldIntervalTime := float64(stableIntervalTime) * ratelimiter.coldFactor
	ratelimiter.thresholdPermits = 0.5 * float64(ratelimiter.warmupPeriodTime) / float64(stableIntervalTime)
	ratelimiter.maxPermits =
		ratelimiter.thresholdPermits + 2.0*float64(ratelimiter.warmupPeriodTime)/(float64(stableIntervalTime)+coldIntervalTime)
	ratelimiter.slope = (coldIntervalTime - float64(stableIntervalTime)) / (ratelimiter.maxPermits - ratelimiter.thresholdPermits)
	if math.IsInf(oldMaxPermits, 1) {
		ratelimiter.storedPermits = 0.0
	} else {
		if oldMaxPermits == 0.0 {
			ratelimiter.storedPermits = ratelimiter.maxPermits
		} else {
			ratelimiter.storedPermits = ratelimiter.storedPermits * ratelimiter.maxPermits / oldMaxPermits
		}
	}
}

func (ratelimiter *SmoothWarmingUp) storedPermitsToWaitTime(storedPermits float64, permitsToTake float64) time.Duration {
	availablePermitsAboveThreshold := storedPermits - ratelimiter.thresholdPermits
	var t time.Duration = 0
	if availablePermitsAboveThreshold > 0.0 {
		permitsAboveThresholdToTake := math.Min(availablePermitsAboveThreshold, permitsToTake)
		length :=
			ratelimiter.permitsToTime(availablePermitsAboveThreshold) + ratelimiter.permitsToTime(availablePermitsAboveThreshold-permitsAboveThresholdToTake)
		t = time.Duration(permitsAboveThresholdToTake * float64(length) / 2.0)
		permitsToTake -= permitsAboveThresholdToTake
	}
	t += ratelimiter.stableIntervalTime * time.Duration(permitsToTake)
	return t
}

func (ratelimiter *SmoothWarmingUp) permitsToTime(permits float64) time.Duration {
	return ratelimiter.stableIntervalTime + time.Duration(permits*ratelimiter.slope)
}

func (ratelimiter *SmoothWarmingUp) coolDownIntervalTime() float64 {
	return float64(ratelimiter.warmupPeriodTime) / ratelimiter.maxPermits
}

func (ratelimiter *SmoothWarmingUp) saturatedAdd(a time.Duration, b time.Duration) time.Duration {
	naiveSum := a + b
	if (a^b) < 0 || (a^naiveSum) >= 0 {
		// If a and b have different signs or a has the same sign as the result then there was no
		// overflow, return.
		return naiveSum
	}
	// we did over/under flow, if the sign is negative we should return MAX otherwise MIN
	return math.MaxInt64 + (((naiveSum & (math.MaxInt64 >> 1)) >> (64 - 1)) ^ 1)
}

func (ratelimiter *SmoothWarmingUp) reserveEarliestAvailable(requiredPermits int, nowTime time.Duration) time.Duration {
	ratelimiter.resync(nowTime)
	returnValue := ratelimiter.nextFreeTicketTime
	storedPermitsToSpend := math.Min(float64(requiredPermits), ratelimiter.storedPermits)
	freshPermits := float64(requiredPermits) - storedPermitsToSpend
	waitTime :=
		ratelimiter.storedPermitsToWaitTime(ratelimiter.storedPermits, storedPermitsToSpend) + time.Duration(freshPermits*float64(ratelimiter.stableIntervalTime))

	ratelimiter.nextFreeTicketTime = ratelimiter.saturatedAdd(ratelimiter.nextFreeTicketTime, waitTime)
	ratelimiter.storedPermits -= storedPermitsToSpend
	return returnValue
}
