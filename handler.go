package main

import (
	"log"

	"github.com/miekg/dns"
)

func recurse(w dns.ResponseWriter, req *dns.Msg) {
	if recurseTo == "" {
		dns.HandleFailed(w, req)
		return
	} else {
		c := new(dns.Client)
	Redo:
		if in, _, err := c.Exchange(req, recurseTo); err == nil { // Second return value is RTT
			if in.MsgHdr.Truncated {
				c.Net = "tcp"
				goto Redo
			}

			w.WriteMsg(in)
			return
		} else {
			log.Printf("Recursive error: %+v\n", err)
			dns.HandleFailed(w, req)
			return
		}
	}
}

func handleDNS(w dns.ResponseWriter, req *dns.Msg) {
	// BIND does not support answering multiple questions so we won't
	var zone *Zone
	var name string

	zones.RLock()
	defer zones.RUnlock()

	if len(req.Question) != 1 {
		dns.HandleFailed(w, req)
		return
	} else {
		if zone, name = zones.match(req.Question[0].Name, req.Question[0].Qtype); zone == nil {
			recurse(w, req)
			return
		}
	}

	zones.hits[name]++

	m := new(dns.Msg)
	m.SetReply(req)

	var answerKnown bool
	for _, r := range (*zone)[dns.RR_Header{Name: req.Question[0].Name, Rrtype: req.Question[0].Qtype, Class: req.Question[0].Qclass}] {
		m.Answer = append(m.Answer, r)
		answerKnown = true
	}

	if !answerKnown && recurseTo != "" { // we don't want to handleFailed when recursion is disabled
		recurse(w, req)
		return
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

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
