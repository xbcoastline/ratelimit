package ratelimiter

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

type SmoothRateLimiter struct {
	warmupPeriodTime time.Duration
	slope            float64
	thresholdPermits float64
	coldFactor       float64

	storedPermits      float64
	maxPermits         float64
	stableIntervalTime time.Duration
	nextFreeTicketTime time.Duration

	*BaseLimiter
}

func checkArgument(b bool, errorMessageTemplate string) error {
	if !b {
		log.Println(errorMessageTemplate)
		return errors.New(errorMessageTemplate)
	} else {
		return nil
	}
}

func NewSmoothWarmingUpLimiter(permitsPerSecond float64, warmupPeriod time.Duration, coldFactor float64) (*SmoothRateLimiter, error) {
	if err := checkArgument(warmupPeriod >= 0, fmt.Sprintf("warmupPeriod must not be negative: %d", warmupPeriod)); err != nil {
		return nil, err
	}
	ratelimiter := SmoothRateLimiter{
		warmupPeriodTime:   warmupPeriod,
		coldFactor:         coldFactor,
		nextFreeTicketTime: 0,
	}
	ratelimiter.BaseLimiter = &BaseLimiter{
		stopwatch:        NewSystemWatch(),
		LimiterInterface: &ratelimiter,
	}
	if err := ratelimiter.stopwatch.Start(); err != nil {
		return nil, err
	}
	if err := ratelimiter.SetRate(permitsPerSecond); err != nil {
		return nil, err
	}

	return &ratelimiter, nil
}

func (limiter *SmoothRateLimiter) queryEarliestAvailable(nowTime time.Duration) time.Duration {
	return limiter.nextFreeTicketTime
}

func (limiter *SmoothRateLimiter) doGetRate() float64 {
	return float64(time.Second) / float64(limiter.stableIntervalTime)
}

func (limiter *SmoothRateLimiter) resync(nowTime time.Duration) {
	// if nextFreeTicket is in the past, resync to now
	if nowTime > limiter.nextFreeTicketTime {
		newPermits := float64(nowTime-limiter.nextFreeTicketTime) / limiter.coolDownIntervalTime()
		limiter.storedPermits = math.Min(limiter.maxPermits, limiter.storedPermits+newPermits)
		limiter.nextFreeTicketTime = nowTime
	}
}

func (limiter *SmoothRateLimiter) doSetRate(permitsPerSecond float64, nowTime time.Duration) {
	limiter.resync(nowTime)
	stableIntervalTime := time.Duration(float64(time.Second) / permitsPerSecond)
	limiter.stableIntervalTime = stableIntervalTime

	oldMaxPermits := limiter.maxPermits
	coldIntervalTime := float64(stableIntervalTime) * limiter.coldFactor
	limiter.thresholdPermits = 0.5 * float64(limiter.warmupPeriodTime) / float64(stableIntervalTime)
	limiter.maxPermits =
		limiter.thresholdPermits + 2.0*float64(limiter.warmupPeriodTime)/(float64(stableIntervalTime)+coldIntervalTime)
	limiter.slope = (coldIntervalTime - float64(stableIntervalTime)) / (limiter.maxPermits - limiter.thresholdPermits)
	if math.IsInf(oldMaxPermits, 1) {
		limiter.storedPermits = 0.0
	} else {
		if oldMaxPermits == 0.0 {
			limiter.storedPermits = limiter.maxPermits
		} else {
			limiter.storedPermits = limiter.storedPermits * limiter.maxPermits / oldMaxPermits
		}
	}
}

func (limiter *SmoothRateLimiter) storedPermitsToWaitTime(storedPermits float64, permitsToTake float64) time.Duration {
	availablePermitsAboveThreshold := storedPermits - limiter.thresholdPermits
	var t time.Duration = 0
	if availablePermitsAboveThreshold > 0.0 {
		permitsAboveThresholdToTake := math.Min(availablePermitsAboveThreshold, permitsToTake)
		length :=
			limiter.permitsToTime(availablePermitsAboveThreshold) + limiter.permitsToTime(availablePermitsAboveThreshold-permitsAboveThresholdToTake)
		t = time.Duration(permitsAboveThresholdToTake * float64(length) / 2.0)
		permitsToTake -= permitsAboveThresholdToTake
	}
	t += limiter.stableIntervalTime * time.Duration(permitsToTake)
	return t
}

func (limiter *SmoothRateLimiter) permitsToTime(permits float64) time.Duration {
	return limiter.stableIntervalTime + time.Duration(permits*limiter.slope)
}

func (limiter *SmoothRateLimiter) coolDownIntervalTime() float64 {
	return float64(limiter.warmupPeriodTime) / limiter.maxPermits
}

func (limiter *SmoothRateLimiter) saturatedAdd(a time.Duration, b time.Duration) time.Duration {
	naiveSum := a + b
	if (a^b) < 0 || (a^naiveSum) >= 0 {
		// If a and b have different signs or a has the same sign as the result then there was no
		// overflow, return.
		return naiveSum
	}
	// we did over/under flow, if the sign is negative we should return MAX otherwise MIN
	return math.MaxInt64 + (((naiveSum & (math.MaxInt64 >> 1)) >> (64 - 1)) ^ 1)
}

func (limiter *SmoothRateLimiter) reserveEarliestAvailable(requiredPermits int, nowTime time.Duration) time.Duration {
	limiter.resync(nowTime)
	returnValue := limiter.nextFreeTicketTime
	storedPermitsToSpend := math.Min(float64(requiredPermits), limiter.storedPermits)
	freshPermits := float64(requiredPermits) - storedPermitsToSpend
	waitTime :=
		limiter.storedPermitsToWaitTime(limiter.storedPermits, storedPermitsToSpend) + time.Duration(freshPermits*float64(limiter.stableIntervalTime))

	limiter.nextFreeTicketTime = limiter.saturatedAdd(limiter.nextFreeTicketTime, waitTime)
	limiter.storedPermits -= storedPermitsToSpend
	return returnValue
}
