package embedcron

import (
	"container/heap"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var _ Timer = (*heapTimer)(nil)
var _ Timeout = (*timeout)(nil)

type heapTimer struct {
	minHeaps
	nextCh    <-chan time.Time
	mu        sync.Mutex
	done      chan struct{}
	stop      chan struct{}
	triggerCh chan *minHeapNode // 装一个到期时间绝对值，如果小于这个值，就不触发判断
	minTime   int64
}

func (h *heapTimer) getMinTime() int64 {
	return atomic.LoadInt64(&h.minTime)
}
func (h *heapTimer) setMinTime(t int64) {
	atomic.StoreInt64(&h.minTime, t)
}

type timeout struct {
	timer *heapTimer
	node  *minHeapNode
	err   chan error
	done  chan struct{}
}

func (t *timeout) Err() <-chan error {
	return t.err
}

func (t *timeout) Done() <-chan struct{} {
	return t.done
}

type minHeapNode struct {
	timeout    *timeout
	f          func()        //用户的callback
	absExpire  time.Time     //绝对时间
	userExpire time.Duration //过期时间段
	index      int           //在min heap中的索引，方便删除或者重新推入堆中
	root       *heapTimer
	name       string
}

type minHeaps []*minHeapNode

func (h *heapTimer) Run() {

	//var nextCh <-chan time.Time = nil
	go func() {
		for {
			select {
			case m := <-h.triggerCh:
				h.consumeTrigger(m)
			case <-h.nextCh:
				h.tryPopAndFunc()
			case <-time.After(time.Second * 1):
				h.tryPopAndFunc()
			case <-h.stop:
				close(h.done)
			case <-h.done:
				goto end
			}
		}
	end:
	}()
}

func (h *heapTimer) consumeTrigger(m *minHeapNode) {
	if h.minTime == 0 || m.absExpire.Unix() < h.minTime {
		h.minTime = m.absExpire.Unix()
		h.nextCh = time.After(m.absExpire.Sub(time.Now()))
	}
}
func (h *heapTimer) tryPopAndFunc() {
	if pop := h.tryPop(); pop != nil {
		go func() {
			defer func() {
				pop.timeout.done <- struct{}{}
			}()
			defer func() {
				r := recover()
				if &r != nil {
					err := fmt.Errorf("run task err %v", r)
					select {
					case pop.timeout.err <- err:
					default:
						// 如果无法装载，就跳过
					}
				}
			}()
			pop.f()
		}()
		h.minTime = 0
	}
}
func (h *heapTimer) tryTrigger(m *minHeapNode) {
	select {
	// 因为有缓存空间限制，所以推触发不阻塞
	case h.triggerCh <- m:
	default:
		return
	}
}
func (h *heapTimer) tryPop() *minHeapNode {
	if len(h.minHeaps) == 0 {
		return nil
	}
	if min := h.minHeaps[0]; min.absExpire.Before(time.Now()) {
		node := heap.Pop(&h.minHeaps).(*minHeapNode)
		if len(h.minHeaps) != 0 {
			// 把最小装进去，触发轮转
			h.tryTrigger(h.minHeaps[0])
		}
		return node
	}
	return nil
}

func (h *heapTimer) Stop() {
	h.stop <- struct{}{}
}

func (h *heapTimer) Offer(f func(), duration time.Duration) Timeout {
	select {
	case <-h.done:
		panic(any("cannot add a task to a closed timer"))
	default:
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	m := &minHeapNode{
		name:      getFunctionName(f),
		f:         f,
		absExpire: time.Now().Add(duration),
	}
	heap.Push(&h.minHeaps, m)
	h.tryTrigger(m)
	t := &timeout{
		timer: h,
		node:  m,
		err:   make(chan error, 1),
		done:  make(chan struct{}, 1),
	}
	m.timeout = t
	return t
}
func getFunctionName(fn interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

func (m minHeaps) Len() int           { return len(m) }
func (m minHeaps) Less(i, j int) bool { return m[i].absExpire.Before(m[j].absExpire) }
func (m minHeaps) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
	m[i].index = i
	m[j].index = j
}

func (m *minHeaps) Push(x any) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*m = append(*m, x.(*minHeapNode))
	lastIndex := len(*m) - 1
	(*m)[lastIndex].index = lastIndex
}

func (m *minHeaps) Pop() any {
	old := *m
	n := len(old)
	x := old[n-1]
	*m = old[0 : n-1]
	return x
}

func (t *timeout) Timer() Timer {
	return t.timer
}

func (t *timeout) Func() func() {
	return t.node.f
}

func (t *timeout) Cancel() {
	t.timer.remove(t.node.index)
}
func (h *heapTimer) remove(i int) {
	heap.Remove(&h.minHeaps, i)
}
