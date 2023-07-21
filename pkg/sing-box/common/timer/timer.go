package timer

import (
	"log"
	"time"
)

func Timer(interval time.Duration, cb func(), justNow bool) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	ticker := time.NewTicker(interval)

	if justNow {
		go func() {
			cb()
		}()
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				cb()
			}
		}
	}()
}
