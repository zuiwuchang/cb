package backend

import (
	"context"
	"errors"
	"log/slog"
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
	GetFast(ctx context.Context) (*Dialer, error)
	Get(ctx context.Context) (*Dialer, error)
	Put(dialer *Dialer, used time.Duration)
	Fail(dialer *Dialer)
	Info() []Info
}
type Info struct {
	Count  int          `json:"count"`
	Server []ServerInfo `json:"server"`
}
type ServerInfo struct {
	Duration string   `json:"duration"`
	Count    int      `json:"count"`
	Address  []string `json:"address"`
}
type sortUint []uint

func (a sortUint) Len() int           { return len(a) }
func (a sortUint) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortUint) Less(i, j int) bool { return a[i] < a[j] }
func New(source Source, port uint16, millisecond []uint,
	min, max int,
) Backend {
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
	if min < 10 {
		min = 10
	}
	if max < min*2*3 {
		max = min * 2 * 3
	}
	backend := &defaultBackend{
		source:    source,
		port:      int(port),
		durations: durations,
		min:       min,
		max:       max,
	}
	go backend.Serve()
	return backend
}

type defaultBackend struct {
	source    Source
	port      int
	durations []time.Duration
	max, min  int

	served uint32
	d0, d1 *DialerManager
	locker sync.RWMutex

	rs *rangeServer
	ip net.IP
}

func (b *defaultBackend) GetFast(ctx context.Context) (dialer *Dialer, e error) {
	b.locker.RLock()
	d0, d1 := b.d0, b.d1
	b.locker.RUnlock()

	if d1 != nil {
		dialer = d1.GetFast()
		if dialer != nil {
			return
		}
	}
	if d0 != nil {
		dialer = d0.GetFast()
	}
	if dialer == nil {
		e = ErrNoTarget
		b.asyncServe()
	}
	return
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
		b.asyncServe()
	}
	return
}
func (b *defaultBackend) Fail(dialer *Dialer) {
	b.locker.RLock()
	if b.d1 != nil {
		n := b.d1.Fail(dialer)
		if dialer.level == 0 && n <= b.min*4/3 {
			b.asyncServe()
		}
	}
	if b.d0 != nil {
		b.d0.Fail(dialer)
	}
	b.locker.RUnlock()
}
func (b *defaultBackend) Put(dialer *Dialer, used time.Duration) {
	b.locker.RLock()
	if b.d1 != nil {
		n := b.d1.Delete(dialer, used)
		if dialer.level == 0 && n <= b.min*4/3 {
			b.asyncServe()
		}
	}
	if b.d0 != nil {
		b.d0.Delete(dialer, used)
	}
	b.locker.RUnlock()
}
func (b *defaultBackend) Serve() {
	var (
		fast int
		e    error
	)
	for {
		if !(b.served == 0 && atomic.CompareAndSwapUint32(&b.served, 0, 1)) {
			time.Sleep(time.Minute * 30)
			continue
		}
		fast, e = b.singleServe()
		if e == nil {
			if fast == 0 {
				time.Sleep(time.Minute)
			} else {
				time.Sleep(time.Hour)
			}
		} else {
			slog.Warn(`get servers fail`,
				`error`, e,
				`retrying`, time.Minute,
			)
			time.Sleep(time.Minute)
		}
	}
}

func (b *defaultBackend) asyncServe() {
	if b.served == 0 && atomic.CompareAndSwapUint32(&b.served, 0, 1) {
		go b.singleServe()
	}
}

var errServersNil = errors.New(`servers nil`)

func (b *defaultBackend) singleServe() (fast int, e error) {
	defer atomic.CompareAndSwapUint32(&b.served, 1, 0)
	servers, e := b.source.Servers()
	if e != nil {
		return
	} else if len(servers) == 0 {
		e = errServersNil
		return
	}
	dialers := newDialerManager(b.durations, b.min)
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
	n := runtime.GOMAXPROCS(0) * 2
	if n < 8 {
		n = 8
	}
	slog.Info(`start ping servers`,
		`goroutine`, n,
		`servers`, servers,
	)
	var wait sync.WaitGroup
	wait.Add(n)
	ch := make(chan net.IP)
	done := make(chan struct{})
	var closed int32
	closef := func() {
		if closed == 0 && atomic.CompareAndSwapInt32(&closed, 0, 1) {
			close(done)
		}
	}

	for i := 0; i < n; i++ {
		go func() {
			b.ping(dialers, ch, closef)
			wait.Done()
		}()
	}
	rs := b.rs
	ip := b.ip

	b.rs = nil
	b.ip = nil
	if rs == nil {
		rs = newRangeServer(servers)
		ip = rs.Get()
	}
DONE:
	for ; ip != nil; ip = rs.Get() {
		if ip.IsPrivate() || !ip.IsGlobalUnicast() {
			continue
		}
		select {
		case ch <- ip:
		case <-done:
			b.rs = rs
			b.ip = ip
			break DONE
		}
	}
	close(ch)
	wait.Wait()
	count := dialers.Len()
	fast = dialers.Fast()
	slog.Info(`ping servers finish`,
		`used`, time.Since(last),
		`count`, count,
		`fast`, fast,
		`next`, time.Hour,
		`done`, b.rs != nil,
	)
	if !setDialers && count > 0 {
		b.locker.Lock()
		b.d0 = b.d1
		b.d1 = dialers
		b.locker.Unlock()
	}
	return
}
func (b *defaultBackend) ping(dialers *DialerManager, ch chan net.IP, closef func()) {
	var (
		dialer net.Dialer
		ip     net.IP
		e      error
		c      net.Conn
		last   time.Time
		used   time.Duration
	)
	for ip = range ch {
		if ip.IsMulticast() ||
			!ip.IsGlobalUnicast() {
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
			dialers.Add(&Dialer{
				addr: addr,
				dialer: &websocket.Dialer{
					NetDial: func(network, _ string) (net.Conn, error) {
						return net.Dial(network, addr.String())
					},
					ReadBufferSize:  1024 * 32,
					WriteBufferSize: 1024 * 32,
				},
			}, used)
			if dialers.Len() >= b.max {
				closef()
			}
		}
	}
}
func (b *defaultBackend) Info() (infos []Info) {
	b.locker.RLock()
	d0, d1 := b.d0, b.d1
	b.locker.RUnlock()
	if d1 != nil {
		infos = append(infos, d1.Info())
	}
	if d0 != nil {
		infos = append(infos, d0.Info())
	}
	return infos
}
