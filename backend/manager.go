package backend

import (
	"log/slog"
	"sync/atomic"
	"time"
)

type DialerManager struct {
	min, max time.Duration
	levels   []*Dialers
	count    int64
}

func newDialerManager(durations []time.Duration, min int) *DialerManager {
	n := len(durations)
	levels := make([]*Dialers, n)
	for i := 0; i < n; i++ {
		levels[i] = newDialers(i, durations[i], min)
	}
	return &DialerManager{
		min:    durations[0],
		max:    durations[n-1],
		levels: levels,
	}
}
func (d *DialerManager) Add(dialer *Dialer, used time.Duration) {
	count := len(d.levels)
	for i := 0; i < count; i++ {
		if used <= d.levels[i].duration {
			dialer.level = i
			count, added := d.levels[i].Add(dialer)
			var total int64
			if added {
				total = atomic.AddInt64(&d.count, 1)
			} else {
				total = atomic.LoadInt64(&d.count)
			}
			slog.Info(`add dialer`,
				`dialer`, dialer,
				`used`, used,
				`level`, d.levels[i].duration,
				`count`, count,
				`total`, total,
			)
		}
	}
}
func (d *DialerManager) Fail(dialer *Dialer) int {
	dialers := d.levels[dialer.level]
	count, deleted := dialers.Fail(dialer)
	var total int64
	if deleted {
		total = atomic.AddInt64(&d.count, -1)
	} else {
		total = atomic.LoadInt64(&d.count)
	}
	slog.Info(`dialer fail`,
		`dialer`, dialer,
		`level`, dialers.duration,
		`count`, count,
		`total`, total,
	)
	return count
}
func (d *DialerManager) Delete(dialer *Dialer, used time.Duration) (n int) {
	dialers := d.levels[dialer.level]
	if dialer.level > 1 {
		count, ok := dialers.Delete(dialer)
		if ok {
			slog.Info(`dialer timeout`,
				`dialer`, dialer,
				`level`, dialers.duration,
				`count`, count,
			)
		}
	} else if used > dialers.duration+time.Millisecond*50 {
		if atomic.AddUint32(&dialer.timeout, 1) > 3 {
			count, ok := dialers.Delete(dialer)
			if ok {
				slog.Info(`dialer timeout`,
					`dialer`, dialer,
					`level`, dialers.duration,
					`count`, count,
				)
			}
		}
	} else {
		atomic.StoreUint32(&dialer.timeout, 0)
	}
	return
}

func (d *DialerManager) Len() int {
	return int(atomic.LoadInt64(&d.count))
}
func (d *DialerManager) Random() (dialer *Dialer) {
	for _, level := range d.levels {
		dialer = level.Random()
		if dialer != nil {
			break
		}
	}
	return
}
func (d *DialerManager) Info() (ret Info) {
	ret.Server = make([]ServerInfo, len(d.levels))
	for i, level := range d.levels {
		ret.Server[i].Duration = level.duration.String()
		ret.Server[i].Address = level.List()
		ret.Server[i].Count = len(ret.Server[i].Address)
		ret.Count += ret.Server[i].Count
	}
	return
}
