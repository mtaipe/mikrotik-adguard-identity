package util

import "testing"

func TestNormalizeMAC(t *testing.T) {
	got := NormalizeMAC("ba-af-3d-0a-f9-b0")
	if got != "BA:AF:3D:0A:F9:B0" {
		t.Fatalf("got %q", got)
	}
}

func TestStableClientIDIsCaseInsensitive(t *testing.T) {
	a := StableClientID("Alice")
	b := StableClientID(" alice ")
	if a != b {
		t.Fatalf("IDs differ: %q %q", a, b)
	}
}
