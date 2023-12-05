package backend

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Dialer struct {
	addr    *net.TCPAddr
	dialer  *websocket.Dialer
	level   int
	timeout uint32
}

func (d *Dialer) String() string {
	return d.addr.String()
}
func (d *Dialer) Dial(urlStr string, requestHeader http.Header) (*websocket.Conn, *http.Response, error) {
	return d.dialer.Dial(urlStr, requestHeader)
}
func (d *Dialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (*websocket.Conn, *http.Response, error) {
	return d.dialer.DialContext(ctx, urlStr, requestHeader)
}

type Dialers struct {
	keys   map[string]*Dialer
	items  []*Dialer
	locker sync.RWMutex

	level    int
	duration time.Duration
	min, max int
}

func newDialers(level int, duration time.Duration, min, max int) *Dialers {
	return &Dialers{
		keys:  make(map[string]*Dialer, 100),
		items: make([]*Dialer, 0, 100),

		level:    level,
		duration: duration,
		min:      min,
		max:      max,
	}
}

func (d *Dialers) Add(dialer *Dialer) (count int, added bool) {
	key := dialer.String()
	d.locker.Lock()
	count = len(d.items)
	if count < d.max {
		if _, exists := d.keys[key]; !exists {
			d.keys[key] = dialer
			d.items = append(d.items, dialer)
			count = len(d.items)
			added = true
		}
	}
	d.locker.Unlock()
	// fmt.Println(`--------------------add`, key, added)
	return
}
func (d *Dialers) delete(dialer *Dialer) (deleted bool) {
	key := dialer.String()
	if old, exists := d.keys[key]; exists && old == dialer {
		deleted = true
		delete(d.keys, key)
		n := len(d.items)
		for i := 0; i < n; i++ {
			if d.items[i] == dialer {
				last := n - 1
				if i != last {
					d.items[i] = d.items[last]
				}
				d.items = d.items[:last]
				break
			}
		}
	}
	// fmt.Println(`-------------------delete`, key, deleted)
	return
}
func (d *Dialers) Fail(dialer *Dialer) (n int, deleted bool) {
	d.locker.Lock()
	deleted = d.delete(dialer)
	n = len(d.items)
	d.locker.Unlock()
	return
}
func (d *Dialers) Delete(dialer *Dialer) (n int, deleted bool) {
	d.locker.Lock()
	if len(d.items) > d.min {
		deleted = d.delete(dialer)
	}
	n = len(d.items)
	d.locker.Unlock()
	return
}
func (d *Dialers) Random() (dialer *Dialer) {
	d.locker.RLock()
	n := len(d.items)
	switch n {
	case 0:
	case 1:
		dialer = d.items[0]
	default:
		dialer = d.items[rand.Intn(n)]
	}
	d.locker.RUnlock()
	return
}
func (d *Dialers) Len() int {
	d.locker.RLock()
	n := len(d.items)
	d.locker.RUnlock()
	return n
}
func (d *Dialers) List() (strs []string) {
	d.locker.RLock()
	strs = make([]string, len(d.items))
	for i, dialer := range d.items {
		strs[i] = dialer.String()
	}
	d.locker.RUnlock()
	return
}
