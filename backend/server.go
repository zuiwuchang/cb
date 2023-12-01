package backend

import "net"

type Source interface {
	Servers() ([]Server, error)
}
type Server interface {
	IP() net.IP
	IPNet() *net.IPNet
	Next(net.IP) net.IP
}
type rangeServer struct {
	states []serverState
	i      int
}

func newRangeServer(server []Server) *rangeServer {
	states := make([]serverState, len(server))
	for i := 0; i < len(server); i++ {
		states[i] = serverState{
			ip:     server[i].IP(),
			server: server[i],
		}
	}
	return &rangeServer{
		states: states,
	}
}
func (r *rangeServer) Get() (ip net.IP) {
	for len(r.states) != 0 {
		if r.i >= len(r.states) {
			r.i = 0
		}
		state := r.states[r.i]
		ip = state.Get()
		if ip != nil {
			r.i++
			break
		}
		last := len(r.states) - 1
		if r.i != last {
			r.states[r.i] = r.states[last]
		}
		r.states = r.states[:last]
	}
	return
}

type serverState struct {
	server Server
	ip     net.IP
}

func (s *serverState) Get() (ip net.IP) {
	if s.ip == nil {
		ip = s.server.IP()
		s.ip = ip
		return
	} else {
		ip = s.server.Next(s.ip)
		if ip != nil {
			s.ip = ip
		}
		return ip
	}
}
