package radius

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/util"
)

var (
	startRE = regexp.MustCompile(`(?i)radius,debug,packet\s+tx Accounting-Request with id\s+(\d+)`)
	endRE   = regexp.MustCompile(`(?i)radius,debug,packet\s+rx Accounting-Response with id\s+(\d+)`)
	attrRE  = regexp.MustCompile(`(?i)radius,debug,packet\s+([A-Za-z0-9-]+)\s*=\s*(.*)$`)
)

type packet struct {
	ID        string
	SourceIP  string
	Attrs     map[string]string
	UpdatedAt time.Time
}

type Parser struct {
	mu      sync.Mutex
	packets map[string]*packet
	current map[string]string
	timeout time.Duration
}

func New(timeout time.Duration) *Parser {
	return &Parser{packets: map[string]*packet{}, current: map[string]string{}, timeout: timeout}
}

func key(source, id string) string { return source + "|" + id }

func (p *Parser) Feed(raw, sourceIP string) (models.RadiusEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m := startRE.FindStringSubmatch(raw); len(m) == 2 {
		k := key(sourceIP, m[1])
		p.packets[k] = &packet{ID: m[1], SourceIP: sourceIP, Attrs: map[string]string{}, UpdatedAt: time.Now()}
		p.current[sourceIP] = k
		return models.RadiusEvent{}, false
	}
	if m := attrRE.FindStringSubmatch(raw); len(m) == 3 {
		k := p.current[sourceIP]
		if pkt := p.packets[k]; pkt != nil {
			pkt.Attrs[m[1]] = clean(m[2])
			pkt.UpdatedAt = time.Now()
		}
		return models.RadiusEvent{}, false
	}
	if m := endRE.FindStringSubmatch(raw); len(m) == 2 {
		k := key(sourceIP, m[1])
		pkt := p.packets[k]
		delete(p.packets, k)
		if p.current[sourceIP] == k {
			delete(p.current, sourceIP)
		}
		if pkt == nil {
			return models.RadiusEvent{}, false
		}
		return eventFrom(pkt)
	}
	return models.RadiusEvent{}, false
}

func (p *Parser) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-p.timeout)
	for k, pkt := range p.packets {
		if pkt.UpdatedAt.Before(cutoff) {
			delete(p.packets, k)
			if p.current[pkt.SourceIP] == k {
				delete(p.current, pkt.SourceIP)
			}
		}
	}
}

func clean(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	return v
}

func eventFrom(pkt *packet) (models.RadiusEvent, bool) {
	var typ models.RadiusEventType
	switch pkt.Attrs["Acct-Status-Type"] {
	case "1":
		typ = models.RadiusStart
	case "2":
		typ = models.RadiusStop
	case "3":
		typ = models.RadiusInterim
	default:
		return models.RadiusEvent{}, false
	}
	username := util.NormalizeUsername(pkt.Attrs["User-Name"])
	mac := util.NormalizeMAC(pkt.Attrs["Calling-Station-Id"])
	sid := pkt.Attrs["Acct-Session-Id"]
	if username == "" || mac == "" || sid == "" {
		return models.RadiusEvent{}, false
	}
	nasIP := pkt.Attrs["NAS-IP-Address"]
	if nasIP == "" {
		nasIP = pkt.SourceIP
	}
	return models.RadiusEvent{
		Type: typ, SourceIP: pkt.SourceIP, SessionID: sid, Username: username, MAC: mac,
		NASID: pkt.Attrs["NAS-Identifier"], NASIP: nasIP, NASPortID: pkt.Attrs["NAS-Port-Id"],
	}, true
}
