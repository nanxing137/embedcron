package embedcron

import (
	"fmt"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	scheduler := NewScheduler()
	i := 0
	scheduler.Every(2).Second().Do(func() {
		i++
		fmt.Printf("exec TestScheduler time %v \n", i)
	})
	go func() {
		select {
		case <-time.After(time.Second * 5):
			scheduler.Stop()
		}
	}()

	scheduler.StartBlocking()
}
