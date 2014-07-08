package main

import (
	"encoding/json"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"net/http"
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
		log.Fatal("Error parsing JSON zones file: ", err)
	} else if err != nil {
		log.Println("Error parsing JSON zones file: ", err)
	}
	/*
	    b, err := json.Marshal(&tmpmap)
	   	if err != nil {
	   		log.Fatal("error:", err)
	   	}
	   	log.Println(string(b))
	*/

	zs.m.Lock()
	zs.store = make(map[string]Zone)
	for key, value := range tmpmap {
		key = dns.Fqdn(key)
		if zs.store[key] == nil {
			zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		}
		for _, r := range value {
			rr, err := dns.NewRR(dns.Fqdn(r.Name) + " " + r.Class + " " + r.Type + " " + r.Data)
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: dns.Fqdn(rr.Header().Name), Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
				zs.store[key][key2] = append(zs.store[key][key2], rr)
			} else {
				log.Println("Skipping problematic record: ", r)
			}
		}
	}
	zs.m.Unlock()
	log.Printf("Loaded %d zones in memory", len(zs.store))
}
