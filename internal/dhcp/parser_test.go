package dhcp

import "testing"

func TestParseACK(t *testing.T) {
	raw := `dhcp,debug lan_dhcp on LAN sending ack with id 2906930129 from 192.168.2.1:67 (04:F4:1C:68:65:69) to 192.168.2.21:68 (BA:AF:3D:0A:F9:B0) (network only)`
	ev, ok := Parse(raw)
	if !ok {
		t.Fatal("ACK was not parsed")
	}
	if ev.IP != "192.168.2.21" || ev.MAC != "BA:AF:3D:0A:F9:B0" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}
