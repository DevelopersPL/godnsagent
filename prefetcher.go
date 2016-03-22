package main

import (
	"encoding/json"
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
		return
	}

	zs.apply(tmpmap, true)
	dbWriteZones(tmpmap, true)
	log.Printf("Loaded %d zones in memory", len(tmpmap))
}
