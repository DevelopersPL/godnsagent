package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codegangsta/cli"
	"github.com/miekg/dns"
)

var (
	zones = &ZoneStore{
		store: make(map[string]Zone),
		hits:  make(map[string]uint64),
	}
	zoneUrl     string
	recurseTo   string
	apiKey      string
	buildtime   string
	buildver    string
	loggerFlags int
)

type ZoneStore struct {
	store map[string]Zone
	hits  map[string]uint64
	sync.RWMutex
}

type Zone map[dns.RR_Header][]dns.RR

func (zs *ZoneStore) match(q string, t uint16) (*Zone, string) {
	zs.RLock()
	defer zs.RUnlock()
	var zone *Zone
	var name string
	b := make([]byte, len(q)) // worst case, one label of length q
	off := 0
	end := false
	for {
		l := len(q[off:])
		for i := 0; i < l; i++ {
			b[i] = q[off+i]
			if b[i] >= 'A' && b[i] <= 'Z' {
				b[i] |= ('a' - 'A')
			}
		}
		if z, ok := zs.store[string(b[:l])]; ok { // 'causes garbage, might want to change the map key
			if t != dns.TypeDS {
				return &z, string(b[:l])
			} else {
				// Continue for DS to see if we have a parent too, if so delegate to the parent
				zone = &z
				name = string(b[:l])
			}
		}
		off, end = dns.NextLabel(q, off)
		if end {
			break
		}
	}
	return zone, name
}

func main() {
	app := cli.NewApp()
	app.Name = "godnsagent"
	app.Usage = "Spigu Web Cloud: DNS Server Agent"
	app.Version = buildver + " built " + buildtime
	app.Author = "Daniel Speichert"
	app.Email = "daniel@speichert.pl"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "zones, z", Value: "http://localhost/zones.json",
			Usage: "The URL of zones in JSON format", EnvVar: "ZONES_URL"},
		cli.StringFlag{Name: "listen, l", Value: "0.0.0.0:53",
			Usage: "The IP:PORT to listen on", EnvVar: "LISTEN"},
		cli.StringFlag{Name: "recurse, r", Value: "",
			Usage:  "Pass-through requests that we can't answer to other DNS server (address:port or empty=disabled)",
			EnvVar: "RECURSE_TO"},
		cli.StringFlag{Name: "http-listen", Value: "0.0.0.0:5380",
			Usage: "IP:PORT to listen on for HTTP interface", EnvVar: "HTTP_LISTEN"},
		cli.StringFlag{Name: "key, k", Value: "",
			Usage:  "API key for HTTP notifications",
			EnvVar: "KEY"},
		cli.StringFlag{Name: "ssl-cert", Value: "/etc/nginx/ssl/server.pem",
			Usage: "path to SSL certificate", EnvVar: "SSL_CERT"},
		cli.StringFlag{Name: "ssl-key", Value: "/etc/nginx/ssl/server.key",
			Usage: "path to SSL key", EnvVar: "SSL_KEY"},
		cli.IntFlag{Name: "flags, f", Value: log.LstdFlags,
			Usage:  "Logger flags (see https://golang.org/pkg/log/#pkg-constants)",
			EnvVar: "LOGGER_FLAGS"},
	}

	app.Action = func(c *cli.Context) {
		log.SetFlags(c.Int("falgs"))
		zoneUrl = c.String("zones")
		recurseTo = c.String("recurse")
		apiKey = c.String("key")

		prefetch(zones, true)

		server := &Server{
			listen:   c.String("listen"),
			rTimeout: 5 * time.Second,
			wTimeout: 5 * time.Second,
			zones:    zones,
		}

		server.Run()

		go StartHTTP(c)

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		for {
			select {
			case s := <-sig:
				log.Fatalf("Signal (%d) received, stopping\n", s)
			}
		}
	}

	app.Run(os.Args)
}
