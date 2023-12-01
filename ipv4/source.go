package ipv4

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zuiwuchang/cb/backend"
	"github.com/zuiwuchang/cb/utils"
)

type FileSource struct {
	filename string
	duration time.Duration
	expired  time.Time
	servers  []backend.Server
}

func NewFileSource(filename string, duration time.Duration) (source *FileSource) {
	source = &FileSource{
		filename: filename,
		duration: duration,
	}
	return
}
func (f *FileSource) Next(ip net.IP) net.IP {
	return Next(ip)
}
func (f *FileSource) Servers() (servers []backend.Server, e error) {
	if f.duration > -1 && len(f.servers) != 0 {
		if f.expired.IsZero() || time.Now().Before(f.expired) {
			servers = f.servers
			return
		}
	}
	slog.Info(`get servers from system`, `filename`, f.filename)
	file, e := os.Open(f.filename)
	if e != nil {
		return
	}
	servers = readServers(file)
	file.Close()
	if e != nil {
		return
	}

	if f.duration >= 0 {
		f.servers = servers
		if f.duration > 0 {
			f.expired = time.Now().Add(f.duration)
		}
	}
	return
}
func readServers(r io.Reader) (servers []backend.Server) {
	br := bufio.NewReader(r)
	var (
		e     error
		b     []byte
		s     string
		ip    net.IP
		ipnet *net.IPNet
	)
	for {
		b, e = br.ReadBytes('\n')
		if e != nil {
			if e != io.EOF {
				slog.Warn(`read file fail`,
					`error`, e,
				)
			}
			break
		}
		s = strings.TrimSpace(utils.BytesToString(b))
		if s == `` || strings.HasPrefix(s, `#`) {
			continue
		}
		ip, ipnet, e = net.ParseCIDR(s)
		if e == nil {
			servers = append(servers, backend.Server{
				IP:    ip,
				IPNet: ipnet,
			})
		} else {
			slog.Warn(`parse cidr fail`,
				`error`, e,
				`cidr`, s,
			)
		}
	}
	return
}

type HttpSource struct {
	url      string
	duration time.Duration
	expired  time.Time
	servers  []backend.Server
}

func NewHttpSource(rawURL string, duration time.Duration) (source *HttpSource, e error) {
	_, e = url.ParseRequestURI(rawURL)
	if e != nil {
		return
	}
	source = &HttpSource{
		url:      rawURL,
		duration: duration,
	}
	return
}
func (f *HttpSource) Next(ip net.IP) net.IP {
	return Next(ip)
}
func (f *HttpSource) Servers() (servers []backend.Server, e error) {
	if f.duration > -1 && len(f.servers) != 0 {
		if f.expired.IsZero() || time.Now().Before(f.expired) {
			servers = f.servers
			return
		}
	}
	slog.Info(`get servers from http`, `url`, f.url)
	resp, e := http.Get(f.url)
	if e != nil {
		return
	} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return
	}
	servers = readServers(resp.Body)
	resp.Body.Close()

	if f.duration >= 0 {
		f.servers = servers
		if f.duration > 0 {
			f.expired = time.Now().Add(f.duration)
		}
	}
	return
}
