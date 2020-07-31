package gocron

import (
	"sort"
	"sync"
	"time"
)

// Scheduler struct, the only data member is the list of jobs.
// - implements the sort.Interface{} for sorting jobs, by the time nextRun
type Scheduler struct {
	jobs []*Job // Array store jobs
	mu   sync.RWMutex

	running  bool
	stopChan chan struct{}
}

// NewScheduler creates a new Scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{
		jobs:     make([]*Job, 0),
		running:  false,
		stopChan: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	<-s.StartAsync()
}

func (s *Scheduler) StartAsync() chan struct{} {
	if s.running {
		return s.stopChan
	}

	s.running = true
	ticker := time.NewTicker(time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.runPending()
			case <-s.stopChan:
				ticker.Stop()
				s.running = false
				return
			}
		}
	}()

	return s.stopChan
}

func (s *Scheduler) Stop() {
	if s.running {
		s.stopScheduler()
	}
}

func (s *Scheduler) stopScheduler() {
	s.stopChan <- struct{}{}
}

// Jobs returns the list of Jobs from the Scheduler
func (s *Scheduler) Jobs() []*Job {
	s.mu.RLock()
	jobs := s.jobs
	s.mu.RUnlock()
	return jobs
}

func (s *Scheduler) Len() int {
	return len(s.jobs)
}

func (s *Scheduler) Swap(i, j int) {
	s.jobs[i], s.jobs[j] = s.jobs[j], s.jobs[i]
}

func (s *Scheduler) Less(i, j int) bool {
	return s.jobs[j].nextRun.Unix() >= s.jobs[i].nextRun.Unix()
}

// Get the current runnable jobs, which shouldRun is True
func (s *Scheduler) runnableJobs() []*Job {
	var jobs []*Job
	sort.Sort(s)
	for _, job := range s.jobs {
		if s.shouldRun(job) {
			jobs = append(jobs, job)
		} else {
			break
		}
	}
	return jobs
}

// NextRun datetime when the next job should run.
func (s *Scheduler) NextRun() (*Job, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.jobs) == 0 {
		return nil, time.Now()
	}
	sort.Sort(s)
	return s.jobs[0], s.jobs[0].nextRun
}

// Every schedule a new periodic job with interval
func (s *Scheduler) Every(interval time.Duration) *Job {
	job := NewJob(interval)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)

	return job
}

// runPending runs all the jobs that are scheduled to run.
func (s *Scheduler) runPending() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var onceJobs []*Job
	jobs := s.runnableJobs()
	for _, j := range jobs {
		go j.run()
		j.lastRun = time.Now()
		j.scheduleNextRun()
		if j.once {
			onceJobs = append(onceJobs, j)
		}
	}
	for _, j := range onceJobs {
		s.RemoveByRef(j)
	}
}

// RunAll run all jobs regardless if they are scheduled to run or not
func (s *Scheduler) RunAll() {
	s.RunAllwithDelay(0)
}

// RunAllwithDelay runs all jobs with delay seconds
func (s *Scheduler) RunAllwithDelay(d time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.jobs {
		go job.run()
		if 0 != d {
			time.Sleep(d)
		}
	}
}

// Remove specific job j by function
func (s *Scheduler) Remove(j interface{}) {
	s.removeByCondition(func(someJob *Job) bool {
		return someJob.jobFunc == getFunctionName(j)
	})
}

// RemoveByRef removes specific job j by reference
func (s *Scheduler) RemoveByRef(j *Job) {
	s.removeByCondition(func(someJob *Job) bool {
		return someJob == j
	})
}

func (s *Scheduler) RemoveByTag(tag string) {
	s.removeByCondition(func(j *Job) bool {
		for idx, _ := range j.tags {
			if j.tags[idx] == tag {
				return true
			}
		}
		return false
	})
}

func (s *Scheduler) removeByCondition(shouldRemove func(*Job) bool) {
	retainedJobs := make([]*Job, 0)
	for _, job := range s.jobs {
		if !shouldRemove(job) {
			retainedJobs = append(retainedJobs, job)
		}
	}
	s.jobs = retainedJobs
}

// Scheduled checks if specific job j was already added
func (s *Scheduler) Scheduled(j interface{}) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.jobs {
		if job.jobFunc == getFunctionName(j) {
			return true
		}
	}
	return false
}

// Clear delete all scheduled jobs
func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = make([]*Job, 0)
}

func (s *Scheduler) shouldRun(j *Job) bool {
	return time.Now().After(j.nextRun)
}
