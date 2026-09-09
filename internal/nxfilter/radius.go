package nxfilter

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	codeAccountingRequest  = 4
	codeAccountingResponse = 5

	attrUserName         = 1
	attrFramedIPAddress  = 8
	attrCallingStationID = 31
	attrNASIdentifier    = 32
	attrAcctStatusType   = 40
	attrAcctSessionID    = 44
	attrAcctAuthentic    = 45

	acctStatusStart   = 1
	acctStatusStop    = 2
	acctStatusInterim = 3

	acctAuthenticRADIUS = 1
)

type radiusAttribute struct {
	typ   byte
	value []byte
}

func textAttr(typ byte, value string) radiusAttribute {
	return radiusAttribute{typ: typ, value: []byte(value)}
}

func uint32Attr(typ byte, value uint32) radiusAttribute {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, value)
	return radiusAttribute{typ: typ, value: b}
}

func ipv4Attr(typ byte, value string) (radiusAttribute, error) {
	ip := net.ParseIP(value).To4()
	if ip == nil {
		return radiusAttribute{}, fmt.Errorf("invalid IPv4 address %q", value)
	}
	return radiusAttribute{typ: typ, value: []byte(ip)}, nil
}

func encodeAccountingRequest(identifier byte, secret string, attrs []radiusAttribute) ([]byte, error) {
	attrLen := 0
	for _, a := range attrs {
		if len(a.value) > 253 {
			return nil, fmt.Errorf("RADIUS attribute %d is too long", a.typ)
		}
		attrLen += 2 + len(a.value)
	}
	totalLen := 20 + attrLen
	if totalLen > 4095 {
		return nil, fmt.Errorf("RADIUS packet too large: %d", totalLen)
	}

	packet := make([]byte, totalLen)
	packet[0] = codeAccountingRequest
	packet[1] = identifier
	binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
	// packet[4:20] remains zero for Request Authenticator calculation.
	off := 20
	for _, a := range attrs {
		packet[off] = a.typ
		packet[off+1] = byte(2 + len(a.value))
		copy(packet[off+2:], a.value)
		off += 2 + len(a.value)
	}

	h := md5.New() // RADIUS Accounting requires MD5 by RFC 2866.
	_, _ = h.Write(packet)
	_, _ = h.Write([]byte(secret))
	copy(packet[4:20], h.Sum(nil))
	return packet, nil
}

func validateAccountingResponse(response, request []byte, secret string) error {
	if len(response) < 20 {
		return fmt.Errorf("short RADIUS response")
	}
	if response[0] != codeAccountingResponse {
		return fmt.Errorf("unexpected RADIUS response code %d", response[0])
	}
	if response[1] != request[1] {
		return fmt.Errorf("RADIUS response identifier mismatch")
	}
	length := int(binary.BigEndian.Uint16(response[2:4]))
	if length < 20 || length > len(response) {
		return fmt.Errorf("invalid RADIUS response length %d", length)
	}

	expectedInput := make([]byte, length)
	copy(expectedInput, response[:length])
	copy(expectedInput[4:20], request[4:20])
	h := md5.New() // Response Authenticator calculation is defined by RFC 2866.
	_, _ = h.Write(expectedInput)
	_, _ = h.Write([]byte(secret))
	expected := h.Sum(nil)
	if !equal16(response[4:20], expected) {
		return fmt.Errorf("invalid RADIUS response authenticator")
	}
	return nil
}

func equal16(a, b []byte) bool {
	if len(a) != 16 || len(b) != 16 {
		return false
	}
	var diff byte
	for i := 0; i < 16; i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
