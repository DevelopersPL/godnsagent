package main

import (
	"log"
	"flag"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/miekg/dns"
	)

// only used in JSON
type Record struct {
	Name    string
	Rrtype  string
	Class   string
	Ttl     uint32
	Data    string
}

func prefetch(zs *ZoneStore) {
	var zoneUrl string
	flag.StringVar(&zoneUrl, "z", "http://localhost/zones.json", "The URL of zones in JSON format")
	flag.Parse()

	resp, err := http.Get(zoneUrl)
  	if err != nil {
		log.Fatal(err)
  	}
  	defer resp.Body.Close()
  	body, err := ioutil.ReadAll(resp.Body)
  	if err != nil {
		log.Fatal(err)
  	}

  	tmpmap := make(map[string][]Record)
	err = json.Unmarshal(body, &tmpmap)
	if err != nil {
		log.Fatal("Error parsing JSON zones file: ", err)
  	}
/*
  	b, err := json.Marshal(&tmpmap)
	if err != nil {
		log.Fatal("error:", err)
	}
	log.Println(string(b))
*/

	for key, value := range tmpmap {
		if zs.store[key] == nil {
			zs.store[key] = make(map[dns.RR_Header][]dns.RR)
		}
		for _, r := range value {
			rr, err := dns.NewRR(dns.Fqdn(r.Name) + " " + r.Class + " " + r.Rrtype + " " + r.Data)
			if err == nil {
				rr.Header().Ttl = r.Ttl
				key2 := dns.RR_Header{Name: rr.Header().Name, Rrtype: rr.Header().Rrtype, Class: rr.Header().Class}
	    		zs.store[key][key2] = append(zs.store[key][key2], rr)
	    	} else {
	    		log.Println("Skipping problematic record: ", r)
	    	}
		}
	}
	log.Printf("Loaded %d zones in memory", len(zs.store))
}
