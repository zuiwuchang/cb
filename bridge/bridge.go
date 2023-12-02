package bridge

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/backend"
)

type Bridge struct {
	l        net.Listener
	backend  backend.Backend
	upgrader *websocket.Upgrader
	mux      *http.ServeMux
	cache    int
}

func New(l net.Listener,
	backend backend.Backend,
	cache int,
) *Bridge {
	mux := http.NewServeMux()
	bridge := &Bridge{
		l:       l,
		backend: backend,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024 * 32,
			WriteBufferSize: 1024 * 32,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		mux:   mux,
		cache: cache,
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
	cache := newCache(urlStr, b.backend, b.cache)
	b.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c0, e := b.upgrader.Upgrade(w, r, nil)
		if e != nil {
			slog.Warn(`ws accept fail`, `error`, e)
			return
		}
		defer c0.Close()

		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		c1, e := cache.Dial(ctx)

		cancel()
		if e != nil {
			c0.Close()
			return
		}

		ch := make(chan bool)
		go bridgeWS(c0, c1, ch, false)
		go bridgeWS(c1, c0, ch, true)

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

func bridgeWS(w, r *websocket.Conn, ch chan<- bool, ping bool) {
	for {
		t, p, e := r.ReadMessage()
		if e != nil {
			break
		}
		if t == websocket.PongMessage {
			continue
		}
		e = w.WriteMessage(t, p)
		if e != nil {
			break
		}
		if ping {
			e = w.WriteMessage(websocket.PongMessage, []byte("cerberus is an idea"))
			if e != nil {
				break
			}
		}
	}
	ch <- true
}
