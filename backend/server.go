package backend

import "net"

type Source interface {
	Servers() ([]Server, error)
	Next(net.IP) net.IP
}

type Server struct {
	IP    net.IP
	IPNet *net.IPNet
}
