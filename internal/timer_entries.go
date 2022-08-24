package internal

import (
	"sort"
	"time"
)

// 计时管理
// 参考https://github.com/robfig/cron
// 与cron主要用于任务计划不同,TimerEntries主要用于倒计时的管理
// 如果放在玩家的独立协程中使用,则倒计时的回调可以保证协程安全,使玩家的倒计时回调更简单
//
// example:
//
// go func() {
//   defer timerEntries.Stop()
//   timerEntries := NewTimerEntries()
//   timerEntries.Start()
//   for {
//       select {
//       case timeNow := <-timerEntries.TimerChan():
//            timerEntries.Run(timeNow)
//       case ...
//       }
//   }
// }
type TimerEntries struct {
	entries []*timerEntry
	Timer   *time.Timer
	// 获取当前时间的接口,默认使用time.Now()
	nowFunc func() time.Time
	// resetTime时的最小间隔,默认1秒
	minInterval time.Duration
}

func NewTimerEntries() *TimerEntries {
	return &TimerEntries{
		minInterval: time.Second,
	}
}

func NewTimerEntriesWithArgs(nowFunc func() time.Time, minInterval time.Duration) *TimerEntries {
	return &TimerEntries{
		nowFunc: nowFunc,
		minInterval: minInterval,
	}
}

// 倒计时回调函数
// 返回值:下一次执行的时间间隔,返回0表示该回调不会继续执行
type TimerJob func() time.Duration

// 时间和回调函数
// 参考https://github.com/robfig/cron
type timerEntry struct {
	next time.Time
	job  TimerJob
}

// 指定时间点执行回调
func (this *TimerEntries) AddTimer(t time.Time, f TimerJob) {
	this.addEntry(&timerEntry{t, f})
}

// 现在往后多少时间执行回调
func (this *TimerEntries) After(d time.Duration, f TimerJob) {
	this.addEntry(&timerEntry{this.getNow().Add(d), f})
}

func (this *TimerEntries) addEntry(entry *timerEntry) {
	this.entries = append(this.entries, entry)
	if this.Timer == nil {
		return
	}
	this.resetTime(this.getNow())
}

func (this *TimerEntries) getNow() time.Time {
	if this.nowFunc == nil {
		return time.Now()
	}
	return this.nowFunc()
}

func (this *TimerEntries) sort() {
	sort.Slice(this.entries, func(i, j int) bool {
		return this.entries[i].next.Before(this.entries[j].next)
	})
}

func (this *TimerEntries) resetTime(now time.Time) {
	if len(this.entries) == 0 {
		this.Timer.Reset(time.Hour * 100000)
	} else {
		d := this.entries[0].next.Sub(now)
		if d < this.minInterval {
			d = this.minInterval
		}
		this.Timer.Reset(d)
	}
}

func (this *TimerEntries) Start() {
	this.sort()
	if len(this.entries) == 0 {
		this.Timer = time.NewTimer(time.Hour * 100000)
	} else {
		// 以最快到期的时间间隔创建一个NewTimer
		this.Timer = time.NewTimer(this.entries[0].next.Sub(this.getNow()))
	}
}

func (this *TimerEntries) Stop() {
	if this.Timer != nil {
		this.Timer.Stop()
	}
}

func (this *TimerEntries) TimerChan() <-chan time.Time {
	return this.Timer.C
}

func (this *TimerEntries) Run(now time.Time) bool {
	removed := false
	modified := false
	jobRun := false
	entryCount := len(this.entries)
	for i := 0; i < entryCount; i++ {
		entry := this.entries[i]
		if entry.next.After(now) {
			break
		}
		// job()里面可能执行append(entries,...)
		// 新加的entry下次Run才能被执行
		d := entry.job()
		jobRun = true
		if d > 0 {
			entry.next = now.Add(d)
		} else {
			entry.next = time.Time{} // Zero
			removed = true
		}
		modified = true
	}
	if removed {
		// 删除过期的timer
		for i := len(this.entries) - 1; i >= 0; i-- {
			if this.entries[i].next.IsZero() {
				this.entries = append(this.entries[:i], this.entries[i+1:]...)
			}
		}
	}
	// 重新排序,重置timer
	if modified {
		this.sort()
		this.resetTime(now)
	}
	return jobRun
}
