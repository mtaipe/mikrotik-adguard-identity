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
		PrintTableInterval:            time.Minute,
		IncompleteRadiusPacketTimeout: 30 * time.Second,
	}
}

func TestValidateAcceptsDefaultsShape(t *testing.T) {
	if err := validate(validTestConfig()); err != nil {
		t.Fatalf("validate() unexpected error: %v", err)
	}
}

func TestValidateRejectsWeakKidControlToken(t *testing.T) {
	cfg := validTestConfig()
	cfg.KidControlAPIToken = "too-short"
	if err := validate(cfg); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("validate() error=%v, want minimum token length error", err)
	}
}

func TestValidateAcceptsStrongKidControlToken(t *testing.T) {
	cfg := validTestConfig()
	cfg.KidControlAPIToken = strings.Repeat("a", 32)
	if err := validate(cfg); err != nil {
		t.Fatalf("validate() unexpected error: %v", err)
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := validTestConfig()
	cfg.MikroTikPort = 0
	if err := validate(cfg); err == nil || !strings.Contains(err.Error(), "MIKROTIK_PORT") {
		t.Fatalf("validate() error=%v, want MIKROTIK_PORT error", err)
	}
}

func TestValidateRejectsNonPositiveTickerInterval(t *testing.T) {
	cfg := validTestConfig()
	cfg.ReconcileInterval = 0
	if err := validate(cfg); err == nil || !strings.Contains(err.Error(), "RECONCILE_INTERVAL") {
		t.Fatalf("validate() error=%v, want RECONCILE_INTERVAL error", err)
	}
}

func TestValidateRejectsNegativeNxFilterRefresh(t *testing.T) {
	cfg := validTestConfig()
	cfg.NxFilterEnabled = true
	cfg.NxFilterHost = "192.0.2.10"
	cfg.NxFilterSharedSecret = "secret"
	cfg.NxFilterRefreshInterval = -time.Second
	if err := validate(cfg); err == nil || !strings.Contains(err.Error(), "NXFILTER_REFRESH_INTERVAL") {
		t.Fatalf("validate() error=%v, want NXFILTER_REFRESH_INTERVAL error", err)
	}
}
