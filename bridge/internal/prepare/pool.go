package prepare

import "sync"

var ch = make(chan []byte, 256)
var pool = &sync.Pool{
	New: func() any {
		return make([]byte, MessageLen)
	},
}

func Get() (b []byte) {
	select {
	case b = <-ch:
	default:
		b = pool.Get().([]byte)
	}
	return
}
func Put(b []byte) {
	select {
	case ch <- b[:MessageLen]:
	default:
		pool.Put(b[:MessageLen])
	}
}
