package main

import (
	"fmt"
	"log"
	_ "net/http/pprof"
	"ratelimit/src/ratelimiter"
	"sync/atomic"
	"time"
)

func main() {
	var success int64 = 0
	var fail int64 = 0
	var freq = 100000.0

	rtlimiter, _ := ratelimiter.NewSmoothWarmingUp(freq, time.Second*3, 3.0)
	log.Println(fmt.Sprintf("start"))
	for i := 0; i < 100; i++ {
		go func() {
			for {
				// get,_:=rtlimiter.TryAcquire(1)
				// if get {
				// 	atomic.AddInt64(&success,1)
				// }else{
				// 	atomic.AddInt64(&fail,1)
				// }
				rtlimiter.Acquire(1)
				//log.Println(fmt.Sprintf("sleep %f s",t))
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			log.Println(fmt.Sprintf("success %d s", success))
			log.Println(fmt.Sprintf("fail %d s", fail))
			atomic.StoreInt64(&success, 0)
			atomic.StoreInt64(&fail, 0)

			log.Println("next")
		}
	}

}
