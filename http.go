package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/miekg/dns"
	"golang.org/x/net/idna"
)

// POST||GET /notify
func HTTPNotifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	log.Println("Got HTTP prefetch notification, reloading zones...")
	prefetch(zones, false)
	fmt.Fprintln(w, "ok")
}

// POST /notify/zones
func HTTPNotifyZonesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	log.Println("Got HTTP zone push notification, updating cache...")
	tmpmap := make(map[string][]Record)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading response body: "+err.Error(), 400)
		return
	}
	if err := json.Unmarshal(body, &tmpmap); err != nil {
		http.Error(w, "Error parsing JSON zones file: "+err.Error(), 400)
		return
	}

	zones.Lock()
	defer zones.Unlock()
	for key, value := range tmpmap {
		key = dns.Fqdn(key)
		if cdn, e := idna.ToASCII(key); e == nil {
			key = cdn
		}
		zones.store[key] = make(map[dns.RR_Header][]dns.RR)
		for _, r := range value {
			r.Name = strings.ToLower(r.Name)
			if cdn, e := idna.ToASCII(r.Name); e == nil {
				r.Name = cdn
			}
			rr, err := dns.NewRR(dns.Fqdn(r.Name) + " " + r.Class + " " + r.Type + " " + r.Data)
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: dns.Fqdn(rr.Header().Name), Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
				zones.store[key][key2] = append(zones.store[key][key2], rr)
			} else {
				log.Printf("Skipping problematic record: %+v\nError: %+v\n", r, err)
			}
		}
	}
	fmt.Fprintf(w, "Loaded %d zone(s) in cache\n", len(tmpmap))
	fmt.Fprintln(w, "ok")
	log.Printf("Loaded %d zone(s) in cache\n", len(tmpmap))
}

// GET /hits
func HTTPHitsHandler(w http.ResponseWriter, r *http.Request) {
	zs := zones
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
	}
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}

	zs.RLock()
	defer zs.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json, _ := json.MarshalIndent(zs.hits, "", `   `)
	fmt.Fprintf(w, "%s", json)
}

// GET /zones
func HTTPZonesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
	}
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}

	zones.RLock()
	defer zones.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json, err := json.MarshalIndent(zones.store, "", `   `)
	if err == nil {
		fmt.Fprintf(w, "%s", json)
	} else {
		http.Error(w, err.Error(), 500)
	}
}

func StartHTTP(c *cli.Context) {
	handlers := http.NewServeMux()
	handlers.HandleFunc("/notify", HTTPNotifyHandler)
	handlers.HandleFunc("/notify/zones", HTTPNotifyZonesHandler)
	handlers.HandleFunc("/hits", HTTPHitsHandler)
	handlers.HandleFunc("/zones", HTTPZonesHandler)

	log.Println("Starting HTTP notification listener on", c.String("http-listen"))
	log.Fatal(http.ListenAndServeTLS(c.String("http-listen"),
		c.String("ssl-cert"), c.String("ssl-key"), handlers))

}
