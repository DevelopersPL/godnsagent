package main

import (
	"log"
	"strconv"
	"time"

	"github.com/miekg/dns"
)

type Server struct {
	host     string
	port     int
	rTimeout time.Duration
	wTimeout time.Duration
	zones    *ZoneStore
}

func (s *Server) Addr() string {
	return s.host + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {
	Handler := NewHandler(s.zones)

	tcpHandler := dns.NewServeMux()
	tcpHandler.HandleFunc(".", Handler.DoTCP)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", Handler.DoUDP)

	tcpServer := &dns.Server{Addr: s.Addr(),
		Net:          "tcp",
		Handler:      tcpHandler,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout}

	udpServer := &dns.Server{Addr: s.Addr(),
		Net:          "udp",
		Handler:      udpHandler,
		UDPSize:      65535,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout}

	go s.start(udpServer)
	go s.start(tcpServer)
}

func (s *Server) start(ds *dns.Server) {
	log.Printf("Start %s listener on %s\n", ds.Net, s.Addr())
	err := ds.ListenAndServe()
	if err != nil {
		log.Fatalf("Start %s listener on %s failed:%s", ds.Net, s.Addr(), err.Error())
	}
}
