package embedcron

import (
	"time"
)

type Timer interface {
	Run()  // 运行
	Stop() // 停止所有定时器
	Offer(func(), time.Duration) Timeout
}

type Timeout interface {
	Timer() Timer          // 获取执行器，一般情况没啥用
	Func() func()          // 获取真实任务
	Cancel()               // 从队列中删除任务
	Err() <-chan error     //done or err
	Done() <-chan struct{} //done or err
}
