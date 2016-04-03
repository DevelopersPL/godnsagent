package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/idna"

	"github.com/boltdb/bolt"
	"github.com/codegangsta/cli"
	"github.com/miekg/dns"
)

var (
	db    *bolt.DB
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
	store   map[string]Zone
	hits    map[string]uint64
	hits_mx sync.RWMutex
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

// pre-process and write map[string][]Record into ZoneStore
// optionally removing everything there was before
func (zs *ZoneStore) apply(tmpmap map[string][]Record, flush bool) {
	zs.Lock()
	defer zs.Unlock()

	if flush {
		zs.store = make(map[string]Zone)
	}

	for key, value := range tmpmap {
		key = strings.ToLower(dns.Fqdn(key))
		if cdn, e := idna.ToASCII(key); e == nil {
			key = cdn
		}
		zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		for _, r := range value {
			r.Name = strings.ToLower(dns.Fqdn(r.Name))
			if cdn, e := idna.ToASCII(r.Name); e == nil {
				r.Name = cdn
			}
			rr, err := dns.NewRR(r.Name + " " + r.Class + " " + r.Type + " " + strings.Replace(r.Data, ";", "\\;", -1))
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: rr.Header().Name, Class: rr.Header().Class}
				zs.store[key][key2] = append(zs.store[key][key2], rr)
				key3 := dns.RR_Header{Name: rr.Header().Name, Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
				zs.store[key][key3] = append(zs.store[key][key3], rr)
			} else {
				log.Printf("Skipping problematic record: %+v\nError: %+v\n", r, err)
			}
		}
	}
}

// read zones from BoltDB and return them as map[string][]Record
func dbReadZones() (zones map[string][]Record, err error) {
	zones = make(map[string][]Record)
	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte("zones"))
		if err := b.ForEach(func(k, v []byte) error {
			var records []Record
			if json.Unmarshal(v, &records) == nil {
				zones[string(k)] = records
			}
			return nil
		}); err != nil {
			return err
		}

		return
	})
	return
}

// write zones to BoltDB from map[string][]Record
// optionally flush all zones first
func dbWriteZones(zones map[string][]Record, flush bool) (err error) {
	err = db.Update(func(tx *bolt.Tx) (err error) {
		if flush {
			err := tx.DeleteBucket([]byte("zones"))
			if err != nil {
				fmt.Errorf("Error creating zones bucket: %s", err)
			}
			_, err = tx.CreateBucketIfNotExists([]byte("zones"))
			if err != nil {
				fmt.Errorf("Error creating zones bucket: %s", err)
			}
		}

		b := tx.Bucket([]byte("zones"))
		for domain, records := range zones {
			json, err := json.MarshalIndent(records, "", `   `)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(domain), json); err != nil {
				return err
			}
		}
		return
	})
	return
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
		log.SetFlags(c.Int("flags"))
		zoneUrl = c.String("zones")
		recurseTo = c.String("recurse")
		apiKey = c.String("key")

		// open database
		var err error
		db, err = bolt.Open("/var/cache/godnsagent.db", 0600, &bolt.Options{Timeout: 5 * time.Second})
		if err != nil {
			log.Fatalln("Can't open /var/cache/godnsagent.db: ", err)
		}
		defer db.Close()

		err = db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("zones"))
			if err != nil {
				return fmt.Errorf("Error creating zones bucket: %s", err)
			}
			return nil
		})
		if err != nil {
			log.Fatal("Error initializing database", err)
		}

		if tmpmap, err := dbReadZones(); err == nil {
			zones.apply(tmpmap, false)
		}

		server := &Server{
			listen:   c.String("listen"),
			rTimeout: 5 * time.Second,
			wTimeout: 5 * time.Second,
			zones:    zones,
		}

		server.Run()

		go StartHTTP(c)

		prefetch(zones, false)

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
