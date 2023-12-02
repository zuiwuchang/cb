package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zuiwuchang/cb/backend"
	"github.com/zuiwuchang/cb/bridge"
	"github.com/zuiwuchang/cb/ipv4"
	"github.com/zuiwuchang/cb/version"
)

const App = `cb`

type Route struct {
	path string
	url  string
}

func createRoot() *cobra.Command {
	var (
		v                 bool
		sourceURI         string
		listen            string
		routes            []string
		certFile, keyFile string
		millisecond       []uint
		min, max          int
		cache             int
	)
	cmd := &cobra.Command{
		Use:   App,
		Short: `Establish an Internet connection to cloudflare`,
		Run: func(cmd *cobra.Command, args []string) {
			if v {
				fmt.Println(version.Platform)
				fmt.Println(version.Version)
				fmt.Println(version.Commit)
				fmt.Println(version.Date)
				fmt.Println(`Use "` + App + ` --help" for more information about this program.`)
				return
			}
			var port uint16
			var items []Route
			for _, s := range routes {
				strs := strings.SplitN(s, `:`, 2)
				if len(strs) != 2 {
					slog.Error(`route invalid`,
						`route`, s,
					)
					os.Exit(1)
				}
				s0 := strings.TrimSpace(strs[0])
				s1 := strings.TrimSpace(strs[1])
				if s0 == `` {
					slog.Error(`route invalid`,
						`route`, s,
					)
					os.Exit(1)
				}

				u, e := url.ParseRequestURI(s1)
				if e != nil {
					slog.Error(`connect websocket url invalid`,
						`route`, s,
						`error`, e,
					)
					os.Exit(1)
				}
				switch u.Scheme {
				case `ws`:
					s := u.Port()
					if s != `` && s != `80` {
						slog.Error(`ws port must be 80`)
						os.Exit(1)
					}
					if port == 0 {
						port = 80
					} else {
						slog.Error(`cannot mix ws and wss`)
						os.Exit(1)
					}
				case `wss`:
					s := u.Port()
					if s != `` && s != `443` {
						slog.Error(`ws port must be 443`)
						os.Exit(1)
					}
					if port == 0 {
						port = 443
					} else {
						slog.Error(`cannot mix ws and wss`)
						os.Exit(1)
					}
				default:
					slog.Error(`backend websocket url invalid`)
					os.Exit(1)
				}

				items = append(items, Route{
					path: s0,
					url:  s1,
				})
			}
			if len(items) == 0 {
				slog.Error(`routes nil`)
				os.Exit(1)
			}

			for _, ms := range millisecond {
				if ms > 1000*50 || ms < 10 {
					slog.Error(`millisecond invalid`, `millisecond`, ms)
					os.Exit(1)
				}
			}

			var (
				source backend.Source
				e      error
			)
			if strings.HasPrefix(sourceURI, `http://`) || strings.HasPrefix(sourceURI, `https://`) {
				source, e = ipv4.NewHttpSource(sourceURI, time.Hour)
				if e != nil {
					slog.Error(e.Error())
					os.Exit(1)
				}
			} else {
				source = ipv4.NewFileSource(sourceURI, time.Hour)
			}

			l, e := net.Listen(`tcp`, listen)
			if e != nil {
				slog.Error(`listen fail`, `error`, e)
				os.Exit(1)
			}
			useTLS := certFile != `` && keyFile != ``
			slog.Info(`serve`, `listen`, listen, `port`, port, `tls`, useTLS, `cache`, cache)

			bridge := bridge.New(l,
				backend.New(source, port, millisecond,
					min, max,
				),
				cache,
			)
			for _, item := range items {
				bridge.Handle(item.path, item.url)
			}
			if useTLS {
				e = bridge.ServeTLS(certFile, keyFile)
			} else {
				e = bridge.Serve()
			}
			slog.Error(e.Error())
			os.Exit(1)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&v, `version`, `v`,
		false,
		`display version`,
	)
	flags.StringVarP(&sourceURI, `srouce`,
		`s`,
		`https://www.cloudflare.com/ips-v4`, `cloudflare ipv4 manifest. http get url or system filepath`)
	flags.StringVarP(&listen, `listen`, `l`, `:8964`, `listen addr`)
	flags.StringVar(&certFile, `cert`, ``, `x509 cert file`)
	flags.StringVar(&keyFile, `key`, ``, `x509 key file`)
	flags.StringSliceVarP(&routes,
		`routes`,
		`r`,
		[]string{
			`/ws/v1:wss://v1.example.com/video`,
			`/ws/v2:wss://v2.example.com/audio`,
		},
		`use ':' to separate source and destination routes`)
	flags.UintSliceVarP(&millisecond, `millisecond`, `m`, []uint{
		200,
		300,
		400,
	}, `set the delay in milliseconds to find a server`)
	flags.IntVar(&min, `min`, 50, `minimum number of IPs`)
	flags.IntVar(&max, `max`, 2000, `maximum number of IPs`)
	flags.IntVarP(&cache, `cache`, `c`, 30, `how many connections to cache`)
	return cmd
}

var root = createRoot()

func Execute() error {
	return root.Execute()
}
