package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/codegangsta/cli"
	"github.com/prometheus/client_golang/prometheus"
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

	zones.apply(tmpmap, false)
	dbWriteZones(tmpmap, false)
	fmt.Fprintf(w, "Loaded %d zone(s) in cache\n", len(tmpmap))
	fmt.Fprintln(w, "ok")
	log.Printf("Loaded %d zone(s) in cache\n", len(tmpmap))
}

// GET /hits
func HTTPHitsHandler(w http.ResponseWriter, r *http.Request) {
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
	json, _ := json.MarshalIndent(zones.hits, "", `   `)
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

	tmpmap, err := dbReadZones()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json, err := json.MarshalIndent(tmpmap, "", `   `)
	if err == nil {
		fmt.Fprintf(w, "%s", json)
	} else {
		http.Error(w, err.Error(), 500)
	}
}

// GET /metrics
func HTTPMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
	}

	/* key-checking is currently disabled
	if r.FormValue("key") != apiKey {
		http.Error(w, "Auth failed: incorrect key", 403)
		return
	}
	*/

	prometheus.Handler().ServeHTTP(w, r)
}

func StartHTTP(c *cli.Context) {
	handlers := http.NewServeMux()
	handlers.HandleFunc("/notify", HTTPNotifyHandler)
	handlers.HandleFunc("/notify/zones", HTTPNotifyZonesHandler)
	handlers.HandleFunc("/hits", HTTPHitsHandler)
	handlers.HandleFunc("/zones", HTTPZonesHandler)
	handlers.HandleFunc("/metrics", HTTPMetricsHandler)

	log.Println("Starting HTTP notification listener on", c.String("http-listen"))
	log.Fatal(http.ListenAndServeTLS(c.String("http-listen"),
		c.String("ssl-cert"), c.String("ssl-key"), handlers))
}
