package models

import "time"

type RadiusEventType string

const (
	RadiusStart   RadiusEventType = "start"
	RadiusStop    RadiusEventType = "stop"
	RadiusInterim RadiusEventType = "interim"
)

type RadiusEvent struct {
	Type      RadiusEventType
	SourceIP  string
	SessionID string
	Username  string
	MAC       string
	NASID     string
	NASIP     string
	NASPortID string
}

type RadiusSession struct {
	Key       string
	SessionID string
	Username  string
	MAC       string
	NASID     string
	NASIP     string
	NASPortID string
	UpdatedAt time.Time
}

type DHCPEvent struct {
	IP  string
	MAC string
}

type DHCPBinding struct {
	IP        string
	MAC       string
	UpdatedAt time.Time
}
