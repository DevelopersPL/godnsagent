package main

import (
	"encoding/json"
	"fmt"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"net/http"
)

func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	log.Println("Got HTTP prefetch notification, reloading zones...")
	prefetch(zones, false)
	fmt.Fprintln(w, "ok")
}

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

	zs.m.Lock()
	for key, value := range tmpmap {
		key = dns.Fqdn(key)
		zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		for _, r := range value {
			rr, err := dns.NewRR(dns.Fqdn(r.Name) + " " + r.Class + " " + r.Type + " " + r.Data)
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: dns.Fqdn(rr.Header().Name), Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
				zs.store[key][key2] = append(zs.store[key][key2], rr)
			} else {
				log.Printf("Skipping problematic record: %+v\n", r)
			}
		}
	}
	zs.m.Unlock()
	fmt.Fprintf(w, "Loaded %d zones into cache\n", len(tmpmap))
	fmt.Fprintln(w, "ok")
}

func StartHTTP() {
	handlers := http.NewServeMux()
	handlers.HandleFunc("/notify", HTTPHandler)
	handlers.HandleFunc("/notify/zones", HTTPZonesHandler)
	httpserver := &http.Server{Addr: listenOn + ":5380", Handler: handlers}
	go httpserver.ListenAndServe()
	log.Println("Start HTTP notification listener on ", listenOn+":5380")
}
