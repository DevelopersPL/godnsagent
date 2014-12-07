package main

import (
	"flag"
	"github.com/miekg/dns"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	zones = &ZoneStore{
		store: make(map[string]Zone),
		m:     new(sync.RWMutex),
	}
	zoneUrl     string
	listenOn    string
	recurseTo   string
	apiKey      string
	buildtime   string
	buildcommit string
)

type ZoneStore struct {
	store map[string]Zone
	m     *sync.RWMutex
}

type Zone map[dns.RR_Header][]dns.RR

func (zs *ZoneStore) match(q string, t uint16) (*Zone, string) {
	zs.m.RLock()
	defer zs.m.RUnlock()
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
				// Continue for DS to see if we have a parent too, if so delegeate to the parent
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
	flag.StringVar(&zoneUrl, "z", "http://localhost/zones.json", "The URL of zones in JSON format")
	flag.StringVar(&listenOn, "l", "", "The IP to listen on (default = blank = ALL)")
	flag.StringVar(&recurseTo, "r", "", "Pass-through requests that we can't answer to other DNS server (address:port or empty=disabled)")
	flag.StringVar(&apiKey, "k", "", "API key for http notifications")
	flag.Parse()

	log.Println("godnsagent (2014) by Daniel Speichert is starting...")
	log.Println("https://github.com/DevelopersPL/godnsagent")
	log.Printf("bult %s from commit %s", buildtime, buildcommit)

	prefetch(zones, true)

	server := &Server{
		host:     listenOn,
		port:     53,
		rTimeout: 5 * time.Second,
		wTimeout: 5 * time.Second,
		zones:    zones,
	}

	server.Run()

	go StartHTTP()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-sig:
			log.Fatalf("Signal (%d) received, stopping\n", s)
		}
	}
}
