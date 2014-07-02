package main

import (
    "time"
    "os"
    "syscall"
    "sync"
    "log"
    "os/signal"
    "github.com/miekg/dns"
)

type ZoneStore struct {
    store     map[string]Zone
    m         *sync.RWMutex
}

type Zone map[dns.RR_Header][]dns.RR

func (zs *ZoneStore) match(q string, t uint16) (*Zone, string) {
    zs.m.RLock()
    defer zs.m.RUnlock()
    var zone *Zone
    var name string
    b := make([]byte, len(q)) // worst case, one label of length q
    off := 0
    end := false
    for {
        l := len(q[off:])
        for i := 0; i < l; i++ {
            b[i] = q[off+i]
            if b[i] >= 'A' && b[i] <= 'Z' {
                b[i] |= ('a' - 'A')
            }
        }
        if z, ok := zs.store[string(b[:l])]; ok { // 'causes garbage, might want to change the map key
            if t != dns.TypeDS {
                return &z, string(b[:l])
            } else {
                // Continue for DS to see if we have a parent too, if so delegeate to the parent
                zone = &z
                name = string(b[:l])
            }
        }
        off, end = dns.NextLabel(q, off)
        if end {
            break
        }
    }
    return zone, name
}

func main() {
    zones := &ZoneStore{
        store:   make(map[string]Zone),
        m:       new(sync.RWMutex),
    }

    prefetch(zones)

    server := &Server{
        host:     "",
        port:     53,
        rTimeout: 5 * time.Second,
        wTimeout: 5 * time.Second,
        zones:    zones,
    }

    server.Run()

    log.Println("godnsagent is running")

    sig := make(chan os.Signal)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    for {
        select {
            case s := <-sig:
                log.Fatalf("Signal (%d) received, stopping\n", s)
        }
    }  
}
