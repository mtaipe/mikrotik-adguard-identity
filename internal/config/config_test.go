package config

import (
	"strings"
	"testing"
	"time"
)

func validTestConfig() Config {
	return Config{
		MikroTikPort:                  8728,
		MikroTikPassword:              "router-password",
		AdGuardPassword:               "adguard-password",
		AdGuardTimeout:                30 * time.Second,
		AdGuardRetryInterval:          30 * time.Second,
		AdGuardSyncDebounce:           2 * time.Second,
		SyslogPort:                    1514,
		NxFilterAccountingPort:        1813,
		NxFilterTimeout:               5 * time.Second,
		NxFilterRefreshInterval:       5 * time.Minute,
		ReconcileInterval:             30 * time.Minute,
		PrintTableInterval:            0,
		LogLevel: "info",
	}
}

func TestValidateDefaultsShape(t *testing.T) {
	if err := validate(validTestConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWeakKidToken(t *testing.T) {
	c := validTestConfig()
	c.KidControlAPIToken = "short"
	if err := validate(c); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateStrongKidToken(t *testing.T) {
	c := validTestConfig()
	c.KidControlAPIToken = strings.Repeat("a", 32)
	if err := validate(c); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePrintIntervalZero(t *testing.T) {
	c := validTestConfig()
	c.PrintTableInterval = 0
	if err := validate(c); err != nil {
		t.Fatal(err)
	}
}
func TestValidateBadLogLevel(t *testing.T) {
	c := validTestConfig()
	c.LogLevel = "trace"
	if err := validate(c); err == nil {
		t.Fatal("expected error")
	}
}
