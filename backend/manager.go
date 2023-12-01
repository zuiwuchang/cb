package backend

import (
	"log/slog"
	"sync/atomic"
	"time"
)

type DialerManager struct {
	min, max time.Duration
	levels   []*Dialers
	count    uint64
}

func newDialerManager(durations []time.Duration) *DialerManager {
	n := len(durations)
	levels := make([]*Dialers, n)
	for i := 0; i < n; i++ {
		levels[i] = newDialers(i, durations[i])
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
			count := d.levels[i].Add(dialer)
			slog.Info(`add dialer`,
				`dialer`, dialer,
				`used`, used,
				`level`, d.levels[i].duration,
				`count`, count,
			)
		}
	}
}
func (d *DialerManager) Fail(dialer *Dialer) {
	dialers := d.levels[dialer.level]
	count := dialers.Fail(dialer)
	slog.Info(`dialer fail`,
		`dialer`, dialer,
		`level`, dialers.duration,
		`count`, count,
	)
}
func (d *DialerManager) Delete(dialer *Dialer, used time.Duration) (n int) {
	dialers := d.levels[dialer.level]
	if used > dialers.duration+time.Millisecond*50 {
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
func (d *DialerManager) Len() (sum int) {
	for _, level := range d.levels {
		sum += level.Len()
	}
	return
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
