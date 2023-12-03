package bridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/backend"
	"github.com/zuiwuchang/cb/bridge/internal/prepare"
)

type Bridge struct {
	l        net.Listener
	backend  backend.Backend
	upgrader *websocket.Upgrader
	mux      *http.ServeMux
	cache    int
	direct   bool
}

func New(l net.Listener,
	backend backend.Backend,
	cache int, direct bool,
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
		mux:    mux,
		cache:  cache,
		direct: direct,
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
	var cache *Cache
	var dialer *websocket.Dialer
	if b.direct {
		dialer = &websocket.Dialer{
			NetDial: func(network, addr string) (net.Conn, error) {
				return net.Dial(network, addr)
			},
			ReadBufferSize:  1024 * 32,
			WriteBufferSize: 1024 * 32,
		}
	} else {
		cache = newCache(urlStr, b.backend, b.cache)
	}
	b.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c0, e := b.upgrader.Upgrade(w, r, nil)
		if e != nil {
			slog.Warn(`ws accept fail`, `error`, e)
			return
		}
		defer c0.Close()
		var c1 *websocket.Conn
		var reader io.Reader
		var readerType int
		if dialer != nil {
			e = b.prepare(c0)
			if e != nil {
				return
			}
			c1, _, e = dialer.Dial(urlStr, nil)
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			c1, e = cache.Dial(ctx)
			cancel()
		}
		if e != nil {
			c0.Close()
			return
		}

		ch := make(chan bool)
		if dialer == nil {
			go bridgeWS(c0, c1, ch, nil, 0)
			go bridgeWS(c1, c0, ch, reader, readerType)
		} else {
			go bridgeWS(c0, c1, ch, nil, 0)
			go bridgeWS(c1, c0, ch, reader, readerType)
		}

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

var errPrepareTimeout = errors.New(`prepare timemout`)
var errUnexpectedType = errors.New(`prepare unexpected type`)

func (b *Bridge) prepare(c *websocket.Conn) (e error) {
	timer := time.NewTimer(time.Second * 5)
	ch := make(chan error, 1)
	go b.prepareRead(c, ch)
	select {
	case <-timer.C:
		e = errPrepareTimeout
	case e = <-ch:
		if !timer.Stop() {
			<-timer.C
		}
		if e != nil {
			return
		}
		e = <-ch
	}
	return
}
func (b *Bridge) prepareRead(c *websocket.Conn, ch chan<- error) {
	var (
		readerType int
		reader     io.Reader
		e          error
		connected  bool
		dst        = prepare.Get()
	)
	for {
		readerType, reader, e = c.NextReader()
		if e != nil {
			ch <- e
			return
		} else if websocket.BinaryMessage != readerType {
			ch <- errUnexpectedType
			return
		}
		e = prepare.Read(dst, reader)
		if e != nil {
			ch <- e
			return
		}
		switch dst[0] {
		case prepare.Connect:
			ch <- nil
			connected = true
		case prepare.Pong:
			if !connected {

			}
		case prepare.Dial:
			if !connected {

			}
		case prepare.ConnectDial:

		}
	}
}

func bridgeWS(w, r *websocket.Conn, ch chan<- bool,
	reader io.Reader, readerType int,
) {
	var (
		dst io.WriteCloser
		e   error
	)
	if reader != nil {
		dst, e = w.NextWriter(readerType)
		if e != nil {
			ch <- true
			return
		}
		_, e = io.Copy(dst, reader)
		if e != nil {
			ch <- true
			return
		}
		e = dst.Close()
		if e != nil {
			ch <- true
			return
		}
	}
	for {
		readerType, reader, e = r.NextReader()
		if e != nil {
			break
		}
		dst, e := w.NextWriter(readerType)
		if e != nil {
			break
		}
		_, e = io.Copy(dst, reader)
		if e != nil {
			break
		}
		e = dst.Close()
		if e != nil {
			break
		}
	}
	ch <- true
}
