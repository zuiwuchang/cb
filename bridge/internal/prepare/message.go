package prepare

import (
	"fmt"
	"io"
)

const MessageLen = 1 + 40

const (
	Connect = iota + 1
	Pong
	Dial
	ConnectDial
)

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
