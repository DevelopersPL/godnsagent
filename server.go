package main

import (
	"log"
	"time"

	"github.com/miekg/dns"
)

type Server struct {
	listen   string
	rTimeout time.Duration
	wTimeout time.Duration
	zones    *ZoneStore
}

func (s *Server) Run() {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", handleDNS)

	tcpServer := &dns.Server{
		Addr:         s.listen,
		Net:          "tcp",
		Handler:      mux,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout,
	}

	udpServer := &dns.Server{
		Addr:         s.listen,
		Net:          "udp",
		Handler:      mux,
		UDPSize:      65535,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout,
	}

	go s.start(udpServer)
	go s.start(tcpServer)
}

func (s *Server) start(ds *dns.Server) {
	log.Printf("Starting %s listener on %s\n", ds.Net, s.listen)
	err := ds.ListenAndServe()
	if err != nil {
		log.Fatalf("Starting %s listener on %s failed:%s", ds.Net, s.listen, err.Error())
	}
}
