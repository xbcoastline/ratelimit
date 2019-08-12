package main

import (
	"coastxie.com/ratelimiter"
	"fmt"
	"time"
)

func main() {
	rl, _ := ratelimiter.NewSlidingLimiter(100000)
	tick := time.Tick(time.Second)
	for j := 0; j < 10; j++ {
		for {
			select {
			case <-tick:
				break
			default:
				if err := rl.Acquire(1); err == nil {
					fmt.Println(time.Now().String() + " success get lock")
				} else {
					fmt.Println(time.Now().String() + " fail get lock " + err.Error())
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
}
