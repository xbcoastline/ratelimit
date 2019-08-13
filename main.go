package main

import (
	"coastxie.com/ratelimiter"
	"fmt"
	"sync/atomic"
	"time"
)

func main() {
	ticketNum := 1000
	rlArray := make([]*ratelimiter.SmoothRateLimiter, ticketNum)
	tickArr := make([]<-chan time.Time, ticketNum)
	sum := make([]int64, ticketNum)
	permitSec := int64(2000)

	for j := 0; j < ticketNum; j++ {
		rlArray[j], _ = ratelimiter.NewSmoothWarmingUpLimiter(float64(permitSec), time.Second*3, 3.0)
		sum[j] = 0
		tickArr[j] = time.Tick(time.Second)
		go func(rl *ratelimiter.SmoothRateLimiter, tick <-chan time.Time, sum *int64, index int) {
			for {
				select {
				case <-tick:
					fmt.Printf("%d:get %d token\n", index, *sum)
					*sum = 0
					break
				default:
					if get, _ := rl.TryAcquire(1, time.Microsecond); get {
						atomic.AddInt64(sum, 1)
					}
				}
				time.Sleep(time.Microsecond)
			}
		}(rlArray[j], tickArr[j], &sum[j], j)
	}
	select {}
}
