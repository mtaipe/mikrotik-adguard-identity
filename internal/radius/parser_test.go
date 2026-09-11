package radius

import "testing"

func TestIsAccountingLog(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"request", `radius,debug,packet tx Accounting-Request with id 38 to 192.168.0.1:1813`, true},
		{"response", `radius,debug,packet rx Accounting-Response with id 38 from 192.168.0.1:1813`, true},
		{"status attribute", `radius,debug,packet     Acct-Status-Type = 3`, true},
		{"non accounting radius", `radius,debug,packet User-Password = "secret"`, false},
		{"dhcp", `dhcp,debug lease bound`, false},
		{"case insensitive", `RADIUS,DEBUG,PACKET TX ACCOUNTING-REQUEST WITH ID 1`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAccountingLog(tt.line); got != tt.want {
				t.Fatalf("IsAccountingLog(%q)=%v want %v", tt.line, got, tt.want)
			}
		})
	}
}
