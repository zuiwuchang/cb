package backend

import (
	"context"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Dialer struct {
	addr   *net.TCPAddr
	dialer *websocket.Dialer
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
	keys   map[*Dialer]bool
	items  []*Dialer
	locker sync.RWMutex
}

func NewDialers() *Dialers {
	return &Dialers{
		keys:  make(map[*Dialer]bool, 100),
		items: make([]*Dialer, 0, 100),
	}
}
func (d *Dialers) Add(dialer *Dialer) {
	var count int
	d.locker.Lock()
	if !d.keys[dialer] {
		d.keys[dialer] = true
		d.items = append(d.items, dialer)
		count = len(d.items)
	}
	d.locker.Unlock()
	slog.Info(`add target`, `target`, dialer.String(), `count`, count)
}
func (d *Dialers) delete(dialer *Dialer) {
	if d.keys[dialer] {
		delete(d.keys, dialer)
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
}
func (d *Dialers) Fail(dialer *Dialer) (n int) {
	d.locker.Lock()
	d.delete(dialer)
	n = len(d.items)
	d.locker.Unlock()
	return
}
func (d *Dialers) Delete(dialer *Dialer) (n int) {
	d.locker.Lock()
	if len(d.items) > 10 {
		d.delete(dialer)
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
