package syslog

import (
	"net"
	"testing"
)

func TestSourceAllowed(t *testing.T) {
	tests := []struct {
		name  string
		ip    net.IP
		rules []string
		want  bool
	}{
		{"nil IP denied", nil, []string{"192.168.0.1"}, false},
		{"exact IPv4 allowed", net.ParseIP("192.168.0.1"), []string{"192.168.0.1"}, true},
		{"exact IPv4 denied", net.ParseIP("192.168.0.2"), []string{"192.168.0.1"}, false},
		{"IPv4 network boundary allowed", net.ParseIP("10.0.0.0"), []string{"10.0.0.0/24"}, true},
		{"IPv4 broadcast boundary allowed", net.ParseIP("10.0.0.255"), []string{"10.0.0.0/24"}, true},
		{"IPv4 outside subnet denied", net.ParseIP("10.0.1.0"), []string{"10.0.0.0/24"}, false},
		{"exact IPv6 allowed", net.ParseIP("2001:db8::1"), []string{"2001:db8::1"}, true},
		{"IPv6 CIDR allowed", net.ParseIP("2001:db8::ffff"), []string{"2001:db8::/64"}, true},
		{"IPv6 CIDR denied", net.ParseIP("2001:db9::1"), []string{"2001:db8::/64"}, false},
		{"empty rules fail closed", net.ParseIP("192.168.0.1"), nil, false},
		{"invalid IP rule ignored", net.ParseIP("192.168.0.1"), []string{"not-an-ip"}, false},
		{"invalid CIDR rule ignored", net.ParseIP("192.168.0.1"), []string{"192.168.0.0/99"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceAllowed(tt.ip, tt.rules); got != tt.want {
				t.Fatalf("sourceAllowed(%v, %v)=%v want %v", tt.ip, tt.rules, got, tt.want)
			}
		})
	}
}
