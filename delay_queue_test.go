package embedcron

import (
	"fmt"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	h := &heapTimer{
		triggerCh: make(chan *minHeapNode, 100),
	}
	h.Offer(func() {
		fmt.Println("3")
		fmt.Println(time.Now())
	}, time.Second*3)
	h.Offer(func() {
		fmt.Println("4")
		fmt.Println(time.Now())
	}, time.Second*4)
	offer := h.Offer(func() {
		fmt.Println(time.Now())
		panic(any("abc"))
	}, time.Second*5)
	h.Offer(func() {
		fmt.Println("1")
		fmt.Println(time.Now())
	}, time.Second)
	h.Offer(func() {
		fmt.Println("10")
		fmt.Println(time.Now())
	}, time.Second*2)

	fmt.Println(h.minHeaps)
	h.Run()
	go func() {
		select {
		case err := <-offer.Err():
			fmt.Printf("err occur")
			fmt.Printf(err.Error())
			fmt.Println(time.Now())
		case <-offer.Done():
			fmt.Printf("done")
			fmt.Println(time.Now())
		}
	}()
	select {
	case <-time.After(time.Second):

	}
}
