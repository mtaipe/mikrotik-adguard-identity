package syslog

import (
	"net"
	"testing"
)

func TestSourceAllowed(t *testing.T) {
	tests := []struct {
		name  string
		ip    string
		rules []string
		want  bool
	}{
		{"exact allowed", "192.168.0.1", []string{"192.168.0.1"}, true},
		{"exact denied", "192.168.0.2", []string{"192.168.0.1"}, false},
		{"cidr allowed", "10.0.0.5", []string{"10.0.0.0/24"}, true},
		{"cidr denied", "10.0.1.5", []string{"10.0.0.0/24"}, false},
		{"empty fail closed", "192.168.0.1", nil, false},
		{"invalid rule ignored", "192.168.0.1", []string{"not-an-ip"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceAllowed(net.ParseIP(tt.ip), tt.rules); got != tt.want {
				t.Fatalf("sourceAllowed(%s, %v)=%v want %v", tt.ip, tt.rules, got, tt.want)
			}
		})
	}
}
