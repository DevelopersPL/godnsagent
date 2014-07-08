package main

import (
	"fmt"
	"log"
	"net/http"
)

func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Got HTTP notification, reloading zones...")
	prefetch(zones, false)
	fmt.Fprintln(w, "ok")
}

func StartHTTP() {
	handlers := http.NewServeMux()
	handlers.HandleFunc("/notify", HTTPHandler)
	httpserver := &http.Server{Addr: listenOn + ":5380", Handler: handlers}
	go httpserver.ListenAndServe()
	log.Println("Start HTTP notification listener on ", listenOn+":5380")
}
