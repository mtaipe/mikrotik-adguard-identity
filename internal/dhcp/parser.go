package dhcp

import (
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/util"
	"regexp"
)

var ackRE = regexp.MustCompile(`(?i)dhcp,debug\s+.*?\bsending ack\b.*?\bto\s+(\d{1,3}(?:\.\d{1,3}){3}):68\s+\(([0-9A-Fa-f:.-]+)\)`)

func Parse(raw string) (models.DHCPEvent, bool) {
	m := ackRE.FindStringSubmatch(raw)
	if len(m) != 3 {
		return models.DHCPEvent{}, false
	}
	return models.DHCPEvent{IP: m[1], MAC: util.NormalizeMAC(m[2])}, true
}
