package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"code.google.com/p/go.net/idna"
	"github.com/miekg/dns"
)

// only used in JSON
type Record struct {
	Name  string
	Type  string
	Class string
	Ttl   uint32
	Data  string
}

func prefetch(zs *ZoneStore, critical bool) {
	log.Printf("Loading zones from %s...\n", zoneUrl)
	resp, err := http.Get(zoneUrl)
	if err != nil && critical {
		log.Fatal(err)
	} else if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil && critical {
		log.Fatal(err)
	} else if err != nil {
		log.Println(err)
	}

	tmpmap := make(map[string][]Record)
	err = json.Unmarshal(body, &tmpmap)
	if err != nil && critical {
		log.Fatal("Error parsing JSON zones file: ", err, string(body))
	} else if err != nil {
		log.Println("Error parsing JSON zones file: ", err)
	}

	zs.Lock()
	zs.store = make(map[string]Zone)
	for key, value := range tmpmap {
		key = dns.Fqdn(key)
		if cdn, e := idna.ToASCII(key); e == nil {
			key = cdn
		}
		if zs.store[key] == nil {
			zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		}
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
	zs.Unlock()
	log.Printf("Loaded %d zones in memory", len(zs.store))
}
