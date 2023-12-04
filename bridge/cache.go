package bridge

import (
	"context"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/backend"
	"github.com/zuiwuchang/cb/bridge/internal/prepare"
)

type Conn struct {
	Last time.Time
	Conn *websocket.Conn
}
type Cache struct {
	backend backend.Backend
	url     string

	list  chan *Conn
	ch    chan *Conn
	cache []*Conn

	sendPing chan *Conn
	endPing  chan bool

	_connect, _pong, _dial, _connectDial []byte
}

func newCache(url string, backend backend.Backend,
	n int, password string,
) (cache *Cache) {
	cache = &Cache{
		backend: backend,
		url:     url,
	}
	if n > 0 {
		b := make([]byte, prepare.MessageLen*2+2)
		b[0] = prepare.Pong
		b[1] = prepare.Dial
		cache._pong = b[:1]
		cache._dial = b[1:2]
		b = b[2:]

		b[0] = prepare.Connect
		copy(b[1:], password)
		cache._connect = b
		b = b[prepare.MessageLen:]
		b[0] = prepare.ConnectDial
		copy(b[1:], password)
		cache._connectDial = b

		cache.ch = make(chan *Conn)
		cache.list = make(chan *Conn, n)
		cache.sendPing = make(chan *Conn)
		cache.endPing = make(chan bool)
		cache.cache = make([]*Conn, 0, n+1)

		go cache.serve()
		go cache.pingGoRoutine()
	}
	return
}

func (c *Cache) serve() {
	var (
		ctx   = context.Background()
		ws    *websocket.Conn
		now   time.Time
		buf   = prepare.Get()
		timer = time.NewTicker(time.Second * 20)
		count int
	)

	for {
		count = len(c.cache)
		if count == 0 {
			select {
			case <-timer.C:
				slog.Info(`cache ping start`, `count`, count+len(c.list))
				c.ping(buf)
				count = len(c.cache)
				slog.Info(`cache ping end`, `count`, count)
			default:
			}
		} else {
			select {
			case <-timer.C:
				slog.Info(`cache ping start`, `count`, count+len(c.list))
				c.ping(buf)
				count = len(c.cache)
				slog.Info(`cache ping end`, `count`, count)
			case c.list <- c.cache[0]:
				count--
				c.cache[0] = c.cache[count]
				c.cache = c.cache[:count]
			}
		}

		if count == 0 {
			dialer, e := c.backend.Get(ctx)
			if e != nil {
				// dialer not exists
				slog.Warn(`cache dialer not exists`, `sleep`, time.Second)
				time.Sleep(time.Second)
				continue
			}
			ws, _, e = dialer.Dial(c.url, nil)
			if e != nil {
				slog.Warn(`cache dial fail`, `error`, e)
				continue
			}
			e = ws.WriteMessage(websocket.BinaryMessage, c._connect)
			if e != nil {
				slog.Warn(`cache connect fail`, `error`, e)
				continue
			}
			now = time.Now()
			c.cache = append(c.cache,
				&Conn{
					Last: now,
					Conn: ws,
				},
			)
			slog.Info(`cache new`, `count`, len(c.cache)+len(c.list))
		}
	}
}
func (c *Cache) ping(buf []byte) {
	var (
		e     error
		node  *Conn
		count = len(c.cache)
		last  int
	)
	if count > 0 {
		for i := count - 1; i >= 0; i-- {
			node = c.cache[i]
			now := time.Now()
			if now.Sub(node.Last) >= time.Second*25 {
				e = c.writeReadPong(node.Conn, buf)
				if e == nil {
					node.Last = now
				} else {
					node.Conn.Close()
					last = len(c.cache) - 1
					c.cache[i] = c.cache[last]
					c.cache = c.cache[:last]
				}
			}
		}
	}

PING_LOOG:
	for {
		count = len(c.cache)
		if count == 0 {
			select {
			case node = <-c.list:
			default:
				return
			}
		} else {
			last = count - 1
			select {
			case node = <-c.list:
			case c.ch <- c.cache[last]:
				c.cache = c.cache[:last]
				continue PING_LOOG
			default:
				return
			}
		}
		now := time.Now()
		if now.Sub(node.Last) >= time.Second*25 {
			if !c.pingOne(node, buf) {
				continue
			}
		}
		c.cache = append(c.cache, node)
	}
}
func (c *Cache) pingOne(node *Conn, buf []byte) (ok bool) {
	var (
		e           error
		count, last int
	)
	for {
		count = len(c.cache)
		if count == 0 {
			e = c.writeReadPong(node.Conn, buf)
			if e == nil {
				node.Last = time.Now()
				ok = true
			} else {
				node.Conn.Close()
			}
			return
		} else {
			last = count - 1
			select {
			case c.sendPing <- node:
				for {
					count = len(c.cache)
					if count == 0 {
						ok = <-c.endPing
						return
					} else {
						last = count - 1
						select {
						case ok = <-c.endPing:
							return
						case c.ch <- c.cache[last]:
							c.cache = c.cache[:last]
						}
					}
				}
			case c.ch <- c.cache[last]:
				c.cache = c.cache[:last]
			}
		}
	}
}
func (c *Cache) pingGoRoutine() {
	var (
		buf = prepare.Get()
		e   error
	)
	for node := range c.sendPing {
		e = c.writeReadPong(node.Conn, buf)
		if e == nil {
			node.Last = time.Now()
			c.endPing <- true
		} else {
			node.Conn.Close()
			c.endPing <- false
		}
	}
}
func (c *Cache) Dial(ctx context.Context) (conn *websocket.Conn, e error) {
	if c.list != nil {
		conn, e = c.get(ctx)
		if e == nil {
			if conn != nil {
				slog.Info(`dial from cache`)
				return
			}
		} else if ctx.Err() != nil {
			return
		}
	}
	for i := 0; i < 2; i++ {
		conn, e = c.dial(ctx)
		if e == nil {
			slog.Info(`dial from net`)
			return
		} else if ctx.Err() != nil {
			return
		}
	}
	return
}
func (c *Cache) get(ctx context.Context) (conn *websocket.Conn, e error) {
	var cache *Conn
	select {
	case <-ctx.Done():
		e = ctx.Err()
		return
	case cache = <-c.list:
	case cache = <-c.ch:
	default:
		return
	}
	conn = cache.Conn
	if time.Since(cache.Last) <= time.Second*25 {
		e = conn.WriteMessage(websocket.BinaryMessage, c._dial)
		if e != nil {
			conn.Close()
			return
		}
		return
	}

	ch := make(chan error, 1)
	go func() {
		ch <- c.writeReadPong(conn, nil)
	}()
	select {
	case <-ctx.Done():
		conn.Close()
		e = ctx.Err()
	case e = <-ch:
		if e != nil {
			conn.Close()
			return
		}
		e = conn.WriteMessage(websocket.BinaryMessage, c._dial)
		if e != nil {
			conn.Close()
			return
		}
	}
	return
}
func (c *Cache) writeReadPong(conn *websocket.Conn, buf []byte) (e error) {
	e = conn.WriteMessage(websocket.BinaryMessage, c._pong)
	if e != nil {
		return
	}
	if len(buf) < 1 {
		buf = prepare.Get()
		defer prepare.Put(buf)
	}
	e = prepare.ReadWebsocket(buf, conn)
	if e != nil {
		return
	}
	if buf[0] != prepare.Pong {
		e = prepare.ErrPong
	}
	return
}
func (c *Cache) dial(ctx context.Context) (conn *websocket.Conn, e error) {
	dialer, e := c.backend.Get(ctx)
	if e != nil {
		slog.Warn(`get dialer fail`, `error`, e)
		return
	}
	last := time.Now()
	conn, _, e = dialer.Dial(c.url, nil)
	if e != nil {
		slog.Warn(`dial fail`,
			`error`, e,
			`dialer`, dialer,
		)
		c.backend.Fail(dialer)
		return
	}
	c.backend.Put(dialer, time.Since(last))

	if c.ch != nil {
		e = conn.WriteMessage(websocket.BinaryMessage, c._connectDial)
		if e != nil {
			conn.Close()
		}
	}
	return
}
