package nxfilter

import (
	"crypto/md5"
	"encoding/binary"
	"testing"
)

func TestAccountingRequestAuthenticator(t *testing.T) {
	ip, err := ipv4Attr(attrFramedIPAddress, "192.168.1.50")
	if err != nil {
		t.Fatal(err)
	}
	p, err := encodeAccountingRequest(7, "secret", []radiusAttribute{
		textAttr(attrUserName, "alice"), ip,
		textAttr(attrNASIdentifier, "identity-sync"),
		uint32Attr(attrAcctStatusType, acctStatusStart),
		textAttr(attrAcctSessionID, "is-test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if p[0] != codeAccountingRequest || p[1] != 7 {
		t.Fatalf("unexpected header: %v", p[:2])
	}
	if got := int(binary.BigEndian.Uint16(p[2:4])); got != len(p) {
		t.Fatalf("length=%d want=%d", got, len(p))
	}

	check := append([]byte(nil), p...)
	for i := 4; i < 20; i++ {
		check[i] = 0
	}
	h := md5.New()
	_, _ = h.Write(check)
	_, _ = h.Write([]byte("secret"))
	want := h.Sum(nil)
	if !equal16(p[4:20], want) {
		t.Fatal("request authenticator mismatch")
	}
}

func TestSessionIDStableAndIPSpecific(t *testing.T) {
	a := Identity{User: "alice", MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.1.10"}
	b := a
	b.IP = "192.168.1.11"
	if sessionID(a) != sessionID(a) {
		t.Fatal("session ID not stable")
	}
	if sessionID(a) == sessionID(b) {
		t.Fatal("session ID should change with IP")
	}
}
