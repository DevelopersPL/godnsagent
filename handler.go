package main

import (
	"github.com/miekg/dns"
)

type DNSHandler struct {
	zones *ZoneStore
}

func NewHandler(zones *ZoneStore) *DNSHandler {
	return &DNSHandler{zones}
}

func (h *DNSHandler) do(Net string, w dns.ResponseWriter, req *dns.Msg) {
	// BIND does not support answering multiple questions so we won't
	var zone *Zone
	var name string
	h.zones.m.RLock()
	defer h.zones.m.RUnlock()

	if len(req.Question) != 1 {
		dns.HandleFailed(w, req)
		return
	} else {
		if zone, name = h.zones.match(req.Question[0].Name, req.Question[0].Qtype); zone == nil {
			dns.HandleFailed(w, req)
			return
		}
	}

	m := new(dns.Msg)
	m.SetReply(req)

	for _, r := range (*zone)[dns.RR_Header{Name: req.Question[0].Name, Rrtype: req.Question[0].Qtype, Class: req.Question[0].Qclass}] {
		m.Answer = append(m.Answer, r)
	}

	// Add Authority section
	for _, r := range (*zone)[dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET}] {
		m.Ns = append(m.Ns, r)

		// Resolve Authority if possible and serve as Extra
		for _, r := range (*zone)[dns.RR_Header{Name: r.(*dns.NS).Ns, Rrtype: dns.TypeA, Class: dns.ClassINET}] {
			m.Extra = append(m.Extra, r)
		}
		for _, r := range (*zone)[dns.RR_Header{Name: r.(*dns.NS).Ns, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}] {
			m.Extra = append(m.Extra, r)
		}
	}

	m.Authoritative = true
	w.WriteMsg(m)
}

func (h *DNSHandler) DoTCP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("tcp", w, req)
}

func (h *DNSHandler) DoUDP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("udp", w, req)
}

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
