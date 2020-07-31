package gocron

import (
	"errors"
	"log"
	"math"
	"reflect"
	"time"
)

var (
	ErrTimeFormat           = errors.New("time format error")
	ErrParamsNotAdapted     = errors.New("the number of params is not adapted")
	ErrNotAFunction         = errors.New("only functions can be schedule into the job queue")
	ErrPeriodNotSpecified   = errors.New("unspecified job period")
	ErrParameterCannotBeNil = errors.New("nil paramaters cannot be used with reflection")
)

// Job struct keeping information about job
type Job struct {
	interval time.Duration
	once     bool
	jobFunc  string    // the job jobFunc to run, func[jobFunc]
	err      error     // error related to job
	lastRun  time.Time // datetime of last run
	nextRun  time.Time // datetime of next run
	from     time.Time
	funcs    map[string]interface{}   // Map for the function task store
	fparams  map[string][]interface{} // Map for function and  params of function
	tags     []string                 // allow the user to tag jobs with certain labels
}

// NewJob creates a new job with the time interval.
func NewJob(interval time.Duration) *Job {
	return &Job{
		interval: interval,
		funcs:    make(map[string]interface{}),
		fparams:  make(map[string][]interface{}),
		tags:     []string{},
	}
}

// True if the job should be run now
func (j *Job) shouldRun() bool {
	return time.Now().Unix() >= j.nextRun.Unix()
}

// Run the job and immediately reschedule it
func (j *Job) run() ([]reflect.Value, error) {
	result, err := callJobFuncWithParams(j.funcs[j.jobFunc], j.fparams[j.jobFunc])
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Err should be checked to ensure an error didn't occur creating the job
func (j *Job) Err() error {
	return j.err
}

// Do specifies the jobFunc that should be called every time the job runs
func (j *Job) Do(jobFun interface{}, params ...interface{}) error {
	if j.err != nil {
		return j.err
	}

	typ := reflect.TypeOf(jobFun)
	if typ.Kind() != reflect.Func {
		return ErrNotAFunction
	}
	fname := getFunctionName(jobFun)
	j.funcs[fname] = jobFun
	j.fparams[fname] = params
	j.jobFunc = fname

	now := time.Now()
	if !j.from.IsZero() && j.from.After(now) {
		j.nextRun = j.from
		return nil
	}

	// without Immediately()
	if j.nextRun.IsZero() {
		j.nextRun = now.Add(j.interval)
	}

	return nil
}

// DoSafely does the same thing as Do, but logs unexpected panics, instead of unwinding them up the chain
// Deprecated: DoSafely exists due to historical compatibility and will be removed soon. Use Do instead
func (j *Job) DoSafely(jobFun interface{}, params ...interface{}) error {
	recoveryWrapperFunc := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Internal panic occurred: %s", r)
			}
		}()

		_, _ = callJobFuncWithParams(jobFun, params)
	}

	return j.Do(recoveryWrapperFunc)
}

func (j *Job) SetTags(tags ...string) {
	j.tags = append(j.tags, tags...)
}

// Untag removes a tag from a job
func (j *Job) Untag(t string) {
	var newTags []string
	for _, tag := range j.tags {
		if t != tag {
			newTags = append(newTags, tag)
		}
	}

	j.tags = newTags
}

// Tags returns the tags attached to the job
func (j *Job) Tags() []string {
	return j.tags
}

// scheduleNextRun Compute the instant when this job should run next
func (j *Job) scheduleNextRun() error {
	now := time.Now()
	if j.lastRun.IsZero() {
		j.lastRun = now
	}

	// advance to next possible schedule
	for j.nextRun.Before(now) || j.nextRun.Before(j.lastRun) {
		j.nextRun = j.nextRun.Add(j.interval)
	}

	return nil
}

// NextScheduledTime returns the time of when this job is to run next
func (j *Job) NextScheduledTime() time.Time {
	return j.nextRun
}

// Immediately job will run at next ticker
func (j *Job) Immediately() *Job {
	j.nextRun = time.Now().Add(-time.Minute)
	return j
}

func (j *Job) Once() *Job {
	j.once = true
	j.interval = math.MaxInt16 * time.Hour
	return j
}

// From schedules the next run of the job
func (j *Job) From(t time.Time) *Job {
	j.from = t
	return j
}

//
func (j *Job) FromMidNight() *Job {
	t := time.Now().Add(24 * time.Hour)
	j.from = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	return j
}
