package main

import (
	"fmt"
	"log"
	"ratelimit/src/ratelimiter"
	"time"
)

func main() {
	var sleep float64
	var err error
	i:=0
	r,err:=ratelimiter.NewSmoothWarmingUp(4,time.Second*3,3.0)
	if err!=nil {
		log.Println(err.Error())
		return
	}
	for{
		sleep,err=r.Acquire(1)
		if err!=nil{
			log.Println(err.Error())
			return
		}
		log.Println("get 1 tokens:"+fmt.Sprintf("%f",sleep)+"s")
		i++
		if i>15 {
			break
		}
	}
}