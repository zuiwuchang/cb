package bridge

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/zuiwuchang/cb/bridge/internal/prepare"
	"github.com/zuiwuchang/cb/utils"
)

func (b *Bridge) prepare(c *websocket.Conn) (e error) {
	timer := time.NewTimer(time.Second * 5)
	ch := make(chan error, 2)
	go b.prepareRead(c, ch)
	select {
	case <-timer.C:
		e = prepare.ErrTimeout
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
		e         error
		connected bool
		dst       = prepare.Get()
	)
	defer prepare.Put(dst)
	for {
		e = prepare.ReadWebsocket(dst, c)
		if e != nil {
			ch <- e
			return
		}
		switch dst[0] {
		case prepare.Connect:
			if utils.BytesToString(dst[1:]) != b.password {
				ch <- prepare.ErrPassword
				return
			}
			ch <- nil
			connected = true
		case prepare.Pong:
			if !connected {
				ch <- prepare.ErrForbidden
				return
			}
			e = c.WriteMessage(websocket.BinaryMessage, dst[:1])
			if e != nil {
				ch <- e
				return
			}
		case prepare.Dial:
			if connected {
				close(ch)
			} else {
				ch <- prepare.ErrForbidden
			}
			return
		case prepare.ConnectDial:
			if utils.BytesToString(dst[1:]) == b.password {
				close(ch)
			} else {
				ch <- prepare.ErrPassword
			}
			return
		}
	}
}
