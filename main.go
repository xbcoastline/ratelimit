package main

import (
	"coastxie.com/ratelimiter"
	"fmt"
	"sync/atomic"
	"time"
)

func main() {
	rl, _ := ratelimiter.NewSlidingLimiter(100000)
	tick := time.Tick(time.Second)
	sum:=int64(0)
	for j := 0; j < 10; j++ {
		for {
			select {
			case <-tick:
				fmt.Printf("get %d token\n",sum)
				sum=0
				break
			default:
				if err := rl.Acquire(1); err == nil {
					atomic.AddInt64(&sum,1)
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
}
