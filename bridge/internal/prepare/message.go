package prepare

import (
	"errors"
	"fmt"
	"io"

	"github.com/gorilla/websocket"
)

const MessageLen = 1 + 40

const (
	Connect = iota + 1
	Pong
	Dial
	ConnectDial
)

var ErrUnexpectedType = errors.New(`prepare: unexpected type`)
var ErrTimeout = errors.New(`prepare: timemout`)
var ErrPassword = errors.New(`prepare: password not matched`)
var ErrForbidden = errors.New(`prepare: forbidden`)
var ErrPong = errors.New(`prepare: expect pong`)

func Read(dst []byte, r io.Reader) (e error) {
	dst = dst[1:MessageLen]
	_, e = r.Read(dst[:1])
	if e != nil {
		return
	}
	switch dst[0] {
	case Connect:
		fallthrough
	case ConnectDial:
		_, e = io.ReadFull(r, dst[1:])
	case Pong:
		fallthrough
	case Dial:
	default:
		e = fmt.Errorf(`unknow prepare type: %v`, dst[0])
	}
	return
}
func ReadWebsocket(dst []byte, ws *websocket.Conn) (e error) {
	t, r, e := ws.NextReader()
	if e != nil {
		return
	} else if t != websocket.BinaryMessage {
		e = ErrUnexpectedType
		return
	}
	e = Read(dst, r)
	return
}
