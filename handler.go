package main

import (
	"log"
	"strings"

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
		m := new(dns.Msg)
		m.SetReply(req)
		m.SetRcodeFormatError(req)
		w.WriteMsg(m)
		return
	}
	req.Question[0].Name = strings.ToLower(req.Question[0].Name)

	zone, name = zones.match(req.Question[0].Name, req.Question[0].Qtype)
	if zone == nil {
		if recurseTo != "" {
			recurse(w, req)
		} else {
			m := new(dns.Msg)
			m.SetReply(req)
			m.SetRcode(req, dns.RcodeNameError)
			w.WriteMsg(m)
		}
		return
	}

	zones.hits[name]++

	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	m.RecursionAvailable = false

	var answerKnown bool
	for _, r := range (*zone)[dns.RR_Header{Name: req.Question[0].Name, Rrtype: req.Question[0].Qtype, Class: req.Question[0].Qclass}] {
		m.Answer = append(m.Answer, r)
		answerKnown = true
	}

	// check for a wildcarad record (*.zone)
	if !answerKnown {
		for _, r := range (*zone)[dns.RR_Header{Name: "*." + name, Rrtype: req.Question[0].Qtype, Class: req.Question[0].Qclass}] {
			r.Header().Name = dns.Fqdn(req.Question[0].Name)
			m.Answer = append(m.Answer, r)
			answerKnown = true
		}
	}

	if !answerKnown && recurseTo != "" { // we don't want to handleFailed when recursion is disabled
		recurse(w, req)
		return
	} else if !answerKnown {
		m.Ns = (*zone)[dns.RR_Header{Name: name, Rrtype: dns.TypeSOA, Class: dns.ClassINET}]
	} else { // answerKnown
		// Add Authority section
		for _, r := range (*zone)[dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET}] {
			m.Ns = append(m.Ns, r)

			// Resolve Authority if possible and serve as ADDITIONAL SECTION
			zone2, _ := zones.match(r.(*dns.NS).Ns, dns.TypeA)
			if zone2 != nil {
				for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.NS).Ns, Rrtype: dns.TypeA, Class: dns.ClassINET}] {
					m.Extra = append(m.Extra, r)
				}
				for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.NS).Ns, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}] {
					m.Extra = append(m.Extra, r)
				}
			}
		}

		// Resolve extra lookups for CNAMEs, SRVs, etc. and put in ADDITIONAL SECTION
		for _, r := range m.Answer {
			switch r.Header().Rrtype {
			case dns.TypeCNAME:
				zone2, _ := zones.match(r.(*dns.CNAME).Target, dns.TypeA)
				if zone2 != nil {
					for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.CNAME).Target, Rrtype: dns.TypeA, Class: dns.ClassINET}] {
						m.Extra = append(m.Extra, r)
					}
					for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.CNAME).Target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}] {
						m.Extra = append(m.Extra, r)
					}
				}
			case dns.TypeSRV:
				zone2, _ := zones.match(r.(*dns.SRV).Target, dns.TypeA)
				if zone2 != nil {
					for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.SRV).Target, Rrtype: dns.TypeA, Class: dns.ClassINET}] {
						m.Extra = append(m.Extra, r)
					}
					for _, r := range (*zone2)[dns.RR_Header{Name: r.(*dns.SRV).Target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}] {
						m.Extra = append(m.Extra, r)
					}
				}
			}
		}
	}
	m.Answer = dns.Dedup(m.Answer, nil)
	m.Extra = dns.Dedup(m.Extra, nil)
	w.WriteMsg(m)
}

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
