# RateLimiter lib
## implement the below rateLimiter
* SmoothRateLimiter
* SlidingWindowLimiter

## detail
### SmoothRateLimiter
based on guava SmoothRateLimiter

```

             ^ throttling
             |
       cold  +                  /
    interval |                 /.
             |                / .
             |               /  .   ← "warmup period" is the area of the trapezoid between
             |              /   .     thresholdPermits and maxPermits
             |             /    .
             |            /     .
             |           /      .
      stable +----------/  WARM .
    interval |          .   UP  .
             |          . PERIOD.
             |          .       .
           0 +----------+-------+--------------→ storedPermits
             0 thresholdPermits maxPermits
```

refer to guava [SmoothRateLimiter](https://github.com/google/guava/blob/master/guava/src/com/google/common/util/concurrent/SmoothRateLimiter.java)

### SlidingWindowLimiter
based on sliding window

refer to [Kong](https://konghq.com/blog/how-to-design-a-scalable-rate-limiting-algorithm/)
##example

### SmoothRateLimiter
```go

```

### SlidingWindowLimiter
```go
package main

import (
	"coastxie.com/ratelimiter"
	"fmt"
	"sync/atomic"
	"time"
)

func main() {
	ticketNum :=1
	rlArray := make([]*ratelimiter.SlidingWindowLimiter,ticketNum)
	tickArr := make([]<-chan time.Time,ticketNum)
	sum:=make([]int64,ticketNum)
	permitSec:=int64(10)

	for j := 0; j < ticketNum; j++ {
		rlArray[j],_=ratelimiter.NewSlidingLimiter(permitSec)
		sum[j]=0
		tickArr[j]=time.Tick(time.Second)
		go func(rl *ratelimiter.SlidingWindowLimiter,tick <-chan time.Time,sum *int64) {
			for {
				select {
				case <-tick:
					fmt.Printf("get %d token\n",sum)
					*sum=0
					break
				default:
					if err := rl.Acquire(1); err == nil {
						atomic.AddInt64(sum,1)
					}
				}
				time.Sleep(time.Microsecond)
			}
		}(rlArray[j],tickArr[j],&sum[j])
	}
	select {

	}
}
```

