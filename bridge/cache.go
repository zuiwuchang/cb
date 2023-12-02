package bridge

import (
	"context"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/backend"
)

type Conn struct {
	Expired time.Time
	Last    time.Time
	Conn    *websocket.Conn
}
type Cache struct {
	backend backend.Backend
	url     string
	list    chan *Conn
	ch      chan *Conn
	cache   []*Conn
}

func newCache(url string, backend backend.Backend, n int) (cache *Cache) {
	var (
		list chan *Conn
		ch   chan *Conn
	)
	if n > 0 {
		list = make(chan *Conn, n)
		ch = make(chan *Conn)
	}
	cache = &Cache{
		backend: backend,
		url:     url,
		list:    list,
		ch:      ch,
		cache:   make([]*Conn, 0, n+1),
	}
	if n > 0 {
		go cache.serve()
		go func() {
			for node := range list {
				ch <- node
			}
		}()
	}
	return
}

func (c *Cache) serve() {
	var (
		ctx = context.Background()
		ws  *websocket.Conn
		now time.Time

		ping  = time.NewTicker(time.Second * 40)
		count int
	)

	for {
		count = len(c.cache)
		if count == 0 {
			select {
			case <-ping.C:
				c.ping()
				count = len(c.cache)
			default:
			}
		} else {
			select {
			case <-ping.C:
				c.ping()
				count = len(c.cache)
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
				time.Sleep(time.Second)
				continue
			}
			ws, _, e = dialer.Dial(c.url, nil)
			if e != nil {
				continue
			}
			now = time.Now()
			c.cache = append(c.cache,
				&Conn{
					Expired: now.Add(time.Hour),
					Last:    now,
					Conn:    ws,
				},
			)
		}
	}
}
func (c *Cache) ping() {
	var e error
	var node *Conn
	count := len(c.cache)
	for i := count - 1; i >= 0; i-- {
		node = c.cache[i]
		now := time.Now()
		if node.Expired.Before(now) {
			node.Conn.Close()
			c.cache[i] = c.cache[len(c.cache)-1]
			c.cache = c.cache[:len(c.cache)-1]
		} else if now.Sub(node.Last) > time.Second*20 {
			e = node.Conn.WriteMessage(websocket.PingMessage, nil)
			if e != nil {
				node.Conn.Close()
				c.cache[i] = c.cache[len(c.cache)-1]
				c.cache = c.cache[:len(c.cache)-1]
			}
		}
	}

	for {
		count = len(c.cache)
		if count == 0 {
			select {
			case node = <-c.list:
			default:
				return
			}
		} else {
			select {
			case node = <-c.list:
			case c.ch <- c.cache[count-1]:
				c.cache = c.cache[:count-1]
			default:
				return
			}
		}
		now := time.Now()
		if node.Expired.Before(now) {
			node.Conn.Close()
			continue
		} else if now.Sub(node.Last) > time.Second*20 {
			e = node.Conn.WriteMessage(websocket.PingMessage, nil)
			if e != nil {
				node.Conn.Close()
				continue
			}
		}
		c.cache = append(c.cache, node)
	}
}
func (c *Cache) Dial(ctx context.Context) (conn *websocket.Conn, e error) {
	if c.ch != nil {
		for i := 0; i < 2; i++ {
			conn, e = c.get(ctx)
			if e == nil {
				return
			} else if ctx.Err() != nil {
				return
			}
		}
	}
	for i := 0; i < 2; i++ {
		conn, e = c.dial(ctx)
		if e == nil {
			return
		} else if ctx.Err() != nil {
			return
		}
	}
	return
}
func (c *Cache) get(ctx context.Context) (conn *websocket.Conn, e error) {
	for {
		select {
		case <-ctx.Done():
			e = ctx.Err()
			return
		case cache := <-c.ch:
			conn = cache.Conn
			now := time.Now()
			if !now.After(cache.Expired) {
				if now.Sub(cache.Last) > time.Second*20 {
					e = conn.WriteMessage(websocket.PingMessage, nil)
				}
				return
			}
		}
	}
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
	return
}