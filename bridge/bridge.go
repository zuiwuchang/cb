package bridge

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/backend"
	"github.com/zuiwuchang/cb/pool"
)

type Bridge struct {
	l        net.Listener
	backend  backend.Backend
	pool     *pool.Pool
	upgrader *websocket.Upgrader
	mux      *http.ServeMux
}

func New(l net.Listener,
	backend backend.Backend,
	pool *pool.Pool,
) *Bridge {
	mux := http.NewServeMux()
	bridge := &Bridge{
		l:       l,
		backend: backend,
		pool:    pool,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024 * 32,
			WriteBufferSize: 1024 * 32,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		mux: mux,
	}
	mux.HandleFunc(`/info`, bridge.info)
	mux.HandleFunc(`/`, bridge.notfound)
	return bridge
}
func (b *Bridge) Serve() (e error) {
	return http.Serve(b.l, b.mux)
}
func (b *Bridge) ServeTLS(certFile, keyFile string) (e error) {
	return http.ServeTLS(b.l, b.mux, certFile, keyFile)
}
func (b *Bridge) Handle(pattern string, urlStr string) {
	slog.Info(`add route`,
		`path`, pattern,
		`connect`, urlStr,
	)
	b.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c0, e := b.upgrader.Upgrade(w, r, nil)
		if e != nil {
			slog.Warn(`ws accept fail`, `error`, e)
			return
		}
		defer c0.Close()

		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		dialer, e := b.backend.Get(ctx)
		cancel()
		if e != nil {
			c0.Close()
			slog.Warn(`get dialer fail`, `error`, e)
			return
		}
		last := time.Now()

		c1, _, e := dialer.Dial(urlStr, nil)
		if e != nil {
			c0.Close()
			slog.Warn(`dial fail`,
				`error`, e,
				`dialer`, dialer,
			)
			b.backend.Fail(dialer)
			return
		}
		b.backend.Put(dialer, time.Since(last))

		ch := make(chan bool)
		go bridgeWS(b.pool, c0, c1, ch)
		go bridgeWS(b.pool, c1, c0, ch)

		<-ch
		timer := time.NewTimer(time.Second)
		select {
		case <-timer.C:
			c0.Close()
			c1.Close()
			<-ch
		case <-ch:
			if !timer.Stop() {
				<-timer.C
			}
			c0.Close()
			c1.Close()
		}
	})
}

func bridgeWS(p *pool.Pool, w, r *websocket.Conn, ch chan<- bool) {
	for {
		t, p, e := r.ReadMessage()
		if e != nil {
			break
		}
		e = w.WriteMessage(t, p)
		if e != nil {
			break
		}
	}
	ch <- true
}
