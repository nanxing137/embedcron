package embedcron

import "time"

type Scheduler struct {
	jobs  []*Job
	stop  chan struct{}
	done  chan struct{}
	timer Timer
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		jobs:  make([]*Job, 0),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		timer: &heapTimer{},
	}
}

type timeUnit int

const (
	seconds timeUnit = iota + 1
	minutes
	hours
	days
	weeks
)

func (tu timeUnit) duration(interval uint64) time.Duration {
	var d time.Duration
	switch tu {
	case seconds:
		d = time.Second
	case minutes:
		d = time.Minute
	case hours:
		d = time.Hour
	case days:
		d = time.Hour * 24
	case weeks:
		d = time.Hour * 24 * 7
	}
	return time.Duration(int64(d) * int64(interval))
}

type Job struct {
	interval uint64
	timeunit timeUnit
	duration time.Duration // 这里的duration也许需要换成一个方法，计算时间周期，来允许cron或者其他不规则时间
	nextTime func() time.Time
	lastRun  time.Time
	nextRun  time.Time
	fun      func()
}

func (s *Scheduler) Every(i uint64) *Scheduler {
	job := NewJob(i)
	s.jobs = append(s.jobs, job)
	return s
}
func (s *Scheduler) Second() *Scheduler {
	job := s.getCurrentJob()
	job.timeunit = seconds
	return s
}
func (s *Scheduler) Do(f func()) *Job {
	job := s.getCurrentJob()
	job.fun = f
	job.duration = job.timeunit.duration(job.interval)
	s.stepFunc(job)
	return job
}

func (s *Scheduler) checkDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}

}
func (s *Scheduler) stepFunc(j *Job) {
	if s.checkDone() {
		return
	}
	s.timer.Offer(func() {
		defer func() {
			s.stepFunc(j)
		}()
		j.fun()
	}, j.duration)
}

// NewJob creates a new Job with the provided interval
func NewJob(interval uint64) *Job {
	return &Job{
		interval: interval,
		lastRun:  time.Time{},
		nextRun:  time.Time{},
	}
}

func (s *Scheduler) getCurrentJob() *Job {
	return s.jobs[len(s.jobs)-1]
}

// StartBlocking starts all the pending jobs using a second-long ticker and blocks the current thread
func (s *Scheduler) StartBlocking() {
	<-s.StartAsync()
}

// StartAsync starts a goroutine that runs all the pending using a second-long ticker
func (s *Scheduler) StartAsync() chan struct{} {
	go func() {
		s.timer.Run()
		for {
			select {
			case <-s.stop:
				close(s.done)
				s.shutdownJobs()
				return
			}
		}
	}()
	return s.done
}

func (s *Scheduler) Stop() {
	s.stop <- struct{}{}
	s.shutdownJobs()
}
func (s *Scheduler) shutdownJobs() {
	s.timer.Stop()
}
