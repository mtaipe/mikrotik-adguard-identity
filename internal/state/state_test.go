package state

import (
	"testing"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
)

func TestSessionKeyPrefersNASIdentifier(t *testing.T) {
	got := sessionKey("NGFW", "192.168.0.1", "82300701")
	if got != "NGFW|82300701" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestSessionKeyFallsBackToNASIP(t *testing.T) {
	got := sessionKey("", "192.168.0.1", "82300701")
	if got != "192.168.0.1|82300701" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestSameSessionIDOnDifferentNASDoesNotCollide(t *testing.T) {
	s := New()
	if !s.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "42", Username: "alice", MAC: "AA-BB-CC-DD-EE-01", NASID: "NAS1", NASIP: "10.0.0.1"}) {
		t.Fatal("first session should change state")
	}
	if !s.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "42", Username: "bob", MAC: "AA-BB-CC-DD-EE-02", NASID: "NAS2", NASIP: "10.0.0.2"}) {
		t.Fatal("second session should change state")
	}
	if len(s.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(s.sessions))
	}
}

func TestKidControlDevicesDeduplicatesMAC(t *testing.T) {
	s := New()
	s.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s1", Username: "Alice", MAC: "AA-BB-CC-DD-EE-01", NASID: "NAS1"})
	s.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s2", Username: "Alice", MAC: "AA:BB:CC:DD:EE:01", NASID: "NAS2"})
	s.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s3", Username: "Bob", MAC: "AA:BB:CC:DD:EE:02", NASID: "NAS1"})

	got := s.KidControlDevices()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].User != "alice" || got[0].MAC != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("unexpected first device: %+v", got[0])
	}
	if got[1].User != "bob" || got[1].MAC != "AA:BB:CC:DD:EE:02" {
		t.Fatalf("unexpected second device: %+v", got[1])
	}
}
