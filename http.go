package main

import (
	"code.google.com/p/go.net/idna"
	"encoding/json"
	"fmt"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"net/http"
)

// POST||GET /notify
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	log.Println("Got HTTP prefetch notification, reloading zones...")
	prefetch(zones, false)
	fmt.Fprintln(w, "ok")
}

// POST /notify/zones
func HTTPZonesHandler(w http.ResponseWriter, r *http.Request) {
	zs := zones
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
	}
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	log.Println("Got HTTP zone push notification, updating cache...")
	tmpmap := make(map[string][]Record)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error parsing JSON zones file: "+err.Error(), 400)
		return
	}
	if err := json.Unmarshal(body, &tmpmap); err != nil {
		http.Error(w, "Error parsing JSON zones file: "+err.Error(), 400)
		return
	}

	zs.Lock()
	defer zs.Unlock()
	for key, value := range tmpmap {
		key = dns.Fqdn(key)
		if cdn, e := idna.ToASCII(key); e == nil {
			key = cdn
		}
		zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		for _, r := range value {
			if cdn, e := idna.ToASCII(r.Name); e == nil {
				r.Name = cdn
			}
			rr, err := dns.NewRR(dns.Fqdn(r.Name) + " " + r.Class + " " + r.Type + " " + r.Data)
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: dns.Fqdn(rr.Header().Name), Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
				zs.store[key][key2] = append(zs.store[key][key2], rr)
			} else {
				log.Printf("Skipping problematic record: %+v\nError: %+v\n", r, err)
			}
		}
	}
	fmt.Fprintf(w, "Loaded %d zones into cache\n", len(tmpmap))
	fmt.Fprintln(w, "ok")
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

func StartHTTP() {
	handlers := http.NewServeMux()
	handlers.HandleFunc("/notify", HTTPHandler)
	handlers.HandleFunc("/notify/zones", HTTPZonesHandler)
	handlers.HandleFunc("/hits", HTTPHitsHandler)
	httpserver := &http.Server{Addr: listenOn + ":5380", Handler: handlers}
	go httpserver.ListenAndServe()
	log.Println("Start HTTP notification listener on ", listenOn+":5380")
}
