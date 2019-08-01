package main

import (
	"fmt"
	"log"
	"ratelimit/src/ratelimiter"
	"sync"
	"time"
)

var mutex sync.Mutex
var count int64

func main() {
	var err error
	r, err := ratelimiter.NewSmoothWarmingUp(500000, time.Second*3, 3.0)
	if err != nil {
		log.Println(err.Error())
		return
	}
	go ticket()
	for i := 1; i <= 200; i++ {
		go acquire(r)
	}
	select {}
}

func acquire(r *ratelimiter.SmoothWarmingUp) {
	for {
		_, err := r.Acquire(1)
		if err != nil {
			log.Println(err.Error())
			return
		}
		mutex.Lock()
		count++
		mutex.Unlock()
	}
}

func ticket() {
	d := time.Duration(time.Second)

	t := time.NewTicker(d)
	defer t.Stop()
	count = 0
	for {
		<-t.C
		mutex.Lock()
		log.Println(fmt.Sprintf("get %d", count))
		count = 0
		mutex.Unlock()
	}
}
