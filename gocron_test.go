package gocron_test

import (
	"fmt"
	"math"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kakami/gocron"
)

func Test_gocron(t *testing.T) {
	mt := &mtt{
		s: gocron.NewScheduler(),
		t: time.Now(),
	}
	s := mt.s
	s.Every(5*time.Second).Do(mt.interval5s, t)
	s.Every(5*time.Second).Immediately().Do(mt.interval5sImmediately, t)
	mt.from = minuteLater()
	s.Every(5*time.Second).From(mt.from).Do(mt.Interval5sFrom, t)
	s.Every(5*time.Second).Immediately().Once().From(mt.from).Do(mt.Interval5sOnce, t)

	s.StartAsync()

	<-s.StartAsync()
}

type mtt struct {
	s *gocron.Scheduler

	t        time.Time
	from     time.Time
	fromOnce sync.Once

	interval5sCnt            int64
	interval5sImmediatelyCnt int64
}

func minuteLater() time.Time {
	return time.Unix((time.Now().Unix()/60+1)*60, 0)
}

func (mt *mtt) interval5s(t *testing.T) {
	mt.interval5sCnt++
	diff := time.Now().Unix() - mt.t.Unix()
	fmt.Println(">>> interval5s > after", diff)
	if math.Abs(float64(diff-mt.interval5sCnt*5)) > 1 {
		t.Error(">>> !!!", diff, mt.interval5sCnt*5)
	} else {
		fmt.Println("Every is good")
	}
	time.Sleep(10 * time.Second)
}

func (mt *mtt) interval5sImmediately(t *testing.T) {
	diff := time.Now().Unix() - mt.t.Unix()
	fmt.Println(">>> interval5s > after", diff)
	// if diff != mt.interval5sImmediatelyCnt*5 {
	if math.Abs(float64(diff-mt.interval5sImmediatelyCnt*5)) > 1 {
		t.Error(">>> !!!", diff, mt.interval5sImmediatelyCnt*5)
	} else {
		fmt.Println(">>> Immediately is good")
	}
	mt.interval5sImmediatelyCnt++
	time.Sleep(10 * time.Second)
}

func (mt *mtt) Interval5sFrom(t *testing.T) {
	mt.fromOnce.Do(func() {
		now := time.Now()
		if math.Abs(float64(now.Unix()-mt.from.Unix())) > 1 {
			t.Error("from !!!", now.Unix(), mt.from.Unix())
		} else {
			fmt.Println("from is good")
		}
	})
}

func (mt *mtt) Interval5sOnce(t *testing.T) {
	now := time.Now()
	if math.Abs(float64(now.Unix()-mt.from.Unix())) > 1 {
		t.Error("from !!!", now.Unix(), mt.from.Unix())
	} else {
		fmt.Println("from is good")
	}

	time.Sleep(3 * time.Second)
	if mt.s.Scheduled(mt.Interval5sOnce) {
		t.Error("Once !!!")
	} else {
		fmt.Println("Once is good")
	}
}

// go tool pprof http://localhost:6060/debug/pprof/profile
func Test_benchmark(t *testing.T) {
	runtime.GOMAXPROCS(0)
	var g errgroup.Group
	g.Go(func() error {
		return http.ListenAndServe(":6060", nil)
	})
	for i := 0; i < 30000; i++ {
		g.Go(func() error {
			schedule()
			return nil
		})
	}

	g.Wait()
}

func schedule() {
	s := gocron.NewScheduler()
	s.Every(100 * time.Second).Immediately().Do(scheduleDo)
	s.Start()
}

func scheduleDo() {
	// fmt.Println("--- do ---", time.Now().Second())
}
