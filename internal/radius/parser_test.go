package radius

import (
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"testing"
	"time"
)

func TestAccountingPacket(t *testing.T) {
	p := New(30 * time.Second)
	src := "192.168.0.1"
	lines := []string{
		`radius,debug,packet tx Accounting-Request with id 38 to 192.168.0.1:1813`,
		`radius,debug,packet     User-Name = "mtaipe"`,
		`radius,debug,packet     Acct-Session-Id = "82300701"`,
		`radius,debug,packet     Calling-Station-Id = "BA-AF-3D-0A-F9-B0"`,
		`radius,debug,packet     Acct-Status-Type = 2`,
		`radius,debug,packet     NAS-Identifier = "NGFW"`,
		`radius,debug,packet     NAS-IP-Address = 192.168.0.1`,
	}
	for _, line := range lines {
		if _, ok := p.Feed(line, src); ok {
			t.Fatal("event emitted before response")
		}
	}
	ev, ok := p.Feed(`radius,debug,packet rx Accounting-Response with id 38 from 192.168.0.1:1813`, src)
	if !ok {
		t.Fatal("event not emitted")
	}
	if ev.Type != models.RadiusStop || ev.Username != "mtaipe" || ev.MAC != "BA:AF:3D:0A:F9:B0" || ev.NASID != "NGFW" || ev.NASIP != "192.168.0.1" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}
