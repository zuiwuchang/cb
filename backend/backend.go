package backend

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var ErrNoTarget = errors.New(`not found any server`)

type Backend interface {
	Get(ctx context.Context) (*Dialer, error)
	Put(dialer *Dialer, used time.Duration)
	Fail(dialer *Dialer)
}

type sortUint []uint

func (a sortUint) Len() int           { return len(a) }
func (a sortUint) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortUint) Less(i, j int) bool { return a[i] < a[j] }
func New(source Source, port uint16, millisecond []uint) Backend {
	switch len(millisecond) {
	case 0:
		millisecond = []uint{200, 300, 400}
	case 1:
	default:
		sort.Sort(sortUint(millisecond))
	}

	durations := make([]time.Duration, len(millisecond))
	for i := 0; i < len(millisecond); i++ {
		durations[i] = time.Millisecond * time.Duration(millisecond[i])
	}
	backend := &defaultBackend{
		source:    source,
		port:      int(port),
		durations: durations,
	}
	go backend.Serve()
	return backend
}

type defaultBackend struct {
	source    Source
	port      int
	durations []time.Duration

	served uint32
	d0, d1 *Dialers
	locker sync.RWMutex
}

func (b *defaultBackend) Get(ctx context.Context) (dialer *Dialer, e error) {
	b.locker.RLock()
	d0, d1 := b.d0, b.d1
	b.locker.RUnlock()

	if d1 != nil {
		dialer = d1.Random()
		if dialer != nil {
			return
		}
	}
	if d0 != nil {
		dialer = d0.Random()
	}
	if dialer == nil {
		e = ErrNoTarget
		go b.serve()
	}
	return
}
func (b *defaultBackend) Fail(dialer *Dialer) {
	b.locker.RLock()
	if b.d1 != nil {
		n := b.d1.Fail(dialer)
		if n < 10 {
			go b.serve()
		}
	}
	if b.d0 != nil {
		b.d0.Fail(dialer)
	}
	b.locker.RUnlock()
}
func (b *defaultBackend) Put(dialer *Dialer, used time.Duration) {
	if used != 0 && used < time.Millisecond*300 {
		return
	}
	b.locker.RLock()
	if b.d1 != nil {
		b.d1.Delete(dialer)
	}
	if b.d0 != nil {
		n := b.d0.Delete(dialer)
		if n < 10 {
			go b.serve()
		}
	}
	b.locker.RUnlock()
}
func (b *defaultBackend) Serve() {
	var (
		updated bool
		count   int
	)
	for {
		updated, count = b.serve()
		if updated {
			if count > 20 {
				time.Sleep(time.Hour)
			} else if count > 10 {
				time.Sleep(time.Minute * 30)
			} else if count > 5 {
				time.Sleep(time.Minute * 10)
			} else {
				time.Sleep(time.Minute)
			}
		} else {
			time.Sleep(time.Minute)
		}
	}
}

func (b *defaultBackend) serve() (updated bool, count int) {
	if !(b.served == 0 && atomic.CompareAndSwapUint32(&b.served, 0, 1)) {
		return
	}
	servers, e := b.source.Servers()
	if e != nil {
		slog.Warn(`get servers fail`, `error`, e)
		return
	} else if len(servers) == 0 {
		slog.Warn(`servers nil`, `error`, e)
		return
	}
	dialers := NewDialers()
	setDialers := false
	b.locker.Lock()
	if b.d1 == nil {
		b.d1 = dialers
		setDialers = true
	} else {
		if b.d1.Len() < 10 {
			b.d0 = b.d1
			b.d1 = dialers
			setDialers = true
		}
	}
	b.locker.Unlock()

	last := time.Now()
	updated = true
	n := runtime.GOMAXPROCS(0) * 2
	if n < 8 {
		n = 8
	}
	if n > len(servers) {
		n = len(servers)
	}
	slog.Info(`start found targets`,
		`goroutine`, n,
		`servers`, servers,
	)
	var wait sync.WaitGroup
	wait.Add(n)
	ch := make(chan Server)
	for i := 0; i < n; i++ {
		go func() {
			for server := range ch {
				b.ping(dialers, server)
			}
			wait.Done()
		}()
	}

	for i := 0; i < len(servers); i++ {
		ch <- servers[i]
	}
	close(ch)
	wait.Wait()
	slog.Info(`ping servers finish`,
		`used`, time.Since(last),
	)
	count = dialers.Len()
	if !setDialers && count > 0 {
		b.locker.Lock()
		b.d0 = b.d1
		b.d1 = dialers
		b.locker.Unlock()
	}
	return
}
func (b *defaultBackend) ping(dialers *Dialers, server Server) {
	var (
		dialer     net.Dialer
		ipnet      = server.IPNet
		ipnetStr   = ipnet.String()
		e          error
		c          net.Conn
		last       time.Time
		used       time.Duration
		ones, bits = ipnet.Mask.Size()
		total      = int(math.Pow(2, float64((bits - ones))))
		index      = -1
	)
	found := 0
	for ip := server.IP; ip != nil && ipnet.Contains(ip); ip = b.source.Next(ip) {
		index++
		if ipnet.IP.IsMulticast() ||
			!ipnet.IP.IsGlobalUnicast() {
			continue
		}
		addr := &net.TCPAddr{
			IP:   ip,
			Port: b.port,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*400)
		last = time.Now()
		c, e = dialer.DialContext(ctx, addr.Network(), addr.String())
		cancel()
		if e == nil {
			c.Close()
			used = time.Since(last)

			if used < time.Millisecond*400 {
				found++
				slog.Info(`found `+addr.String(),
					`used`, used,
					`found`, found,
					`index`, index,
					`total`, total,
					`ipnet`, ipnetStr,
				)
				dialers.Add(&Dialer{
					addr: addr,
					dialer: &websocket.Dialer{
						NetDial: func(network, _ string) (net.Conn, error) {
							return net.Dial(network, addr.String())
						},
					},
				})
				continue
			}
		}
	}
}
