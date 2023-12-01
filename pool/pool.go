package pool

import "sync"

type Pool struct {
	ch   chan []byte
	cap  int
	pool sync.Pool
}

func New(cap, chanlen int) *Pool {
	if cap < 1 {
		panic(`cap must > 0`)
	}
	var ch chan []byte
	if chanlen > 0 {
		ch = make(chan []byte, chanlen)
	}
	p := &Pool{
		ch:  ch,
		cap: cap,
	}
	p.pool.New = func() any {
		return make([]byte, cap)
	}
	return p
}
func (p *Pool) Get() []byte {
	if p.ch != nil {
		select {
		case b := <-p.ch:
			return b[:p.cap]
		default:
		}
	}
	return p.pool.Get().([]byte)[:p.cap]
}
func (p *Pool) Put(b []byte) {
	if cap(b) < p.cap {
		return
	} else if p.ch != nil {
		select {
		case p.ch <- b:
			return
		default:
		}
	}

	p.pool.Put(b)
}
