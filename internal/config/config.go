package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MikroTikHost     string
	MikroTikPort     int
	MikroTikUser     string
	MikroTikPassword string

	AdGuardURL           string
	AdGuardUser          string
	AdGuardPassword      string
	AdGuardVerifyTLS     bool
	AdGuardTimeout       time.Duration
	AdGuardRetryInterval time.Duration
	AdGuardSyncDebounce  time.Duration

	SyslogAddress        string
	SyslogPort           int
	SyslogAllowedSources []string

	HTTPAddress        string
	KidControlAPIToken string

	NxFilterEnabled         bool
	NxFilterHost            string
	NxFilterAccountingPort  int
	NxFilterSharedSecret    string
	NxFilterNASIdentifier   string
	NxFilterTimeout         time.Duration
	NxFilterRefreshInterval time.Duration

	ReconcileInterval             time.Duration
	PrintTableInterval            time.Duration
	IncompleteRadiusPacketTimeout time.Duration
}

func Load() (Config, error) {
	_ = loadDotEnv(".env")
	cfg := Config{
		MikroTikHost:                  get("MIKROTIK_HOST", "192.168.0.1"),
		MikroTikPort:                  getInt("MIKROTIK_PORT", 8728),
		MikroTikUser:                  get("MIKROTIK_USER", "identitysync"),
		MikroTikPassword:              os.Getenv("MIKROTIK_PASSWORD"),
		AdGuardURL:                    get("ADGUARD_URL", "http://192.168.0.103"),
		AdGuardUser:                   get("ADGUARD_USER", "admin"),
		AdGuardPassword:               os.Getenv("ADGUARD_PASSWORD"),
		AdGuardVerifyTLS:              getBool("ADGUARD_VERIFY_TLS", true),
		AdGuardTimeout:                time.Duration(getInt("ADGUARD_TIMEOUT", 30)) * time.Second,
		AdGuardRetryInterval:          time.Duration(getInt("ADGUARD_RETRY_INTERVAL", 30)) * time.Second,
		AdGuardSyncDebounce:           time.Duration(getInt("ADGUARD_SYNC_DEBOUNCE", 2)) * time.Second,
		SyslogAddress:                 get("SYSLOG_LISTEN_ADDRESS", "0.0.0.0"),
		SyslogPort:                    getInt("SYSLOG_LISTEN_PORT", 1514),
		SyslogAllowedSources:          getCSV("SYSLOG_ALLOWED_SOURCES", get("MIKROTIK_HOST", "192.168.0.1")),
		HTTPAddress:                   get("HTTP_LISTEN_ADDRESS", "0.0.0.0:8080"),
		KidControlAPIToken:            os.Getenv("KID_CONTROL_API_TOKEN"),
		NxFilterEnabled:               getBool("NXFILTER_ENABLED", false),
		NxFilterHost:                  os.Getenv("NXFILTER_HOST"),
		NxFilterAccountingPort:        getInt("NXFILTER_ACCOUNTING_PORT", 1813),
		NxFilterSharedSecret:          os.Getenv("NXFILTER_SHARED_SECRET"),
		NxFilterNASIdentifier:         get("NXFILTER_NAS_IDENTIFIER", "mikrotik-adguard-identity"),
		NxFilterTimeout:               time.Duration(getInt("NXFILTER_TIMEOUT", 5)) * time.Second,
		NxFilterRefreshInterval:       time.Duration(getInt("NXFILTER_REFRESH_INTERVAL", 300)) * time.Second,
		ReconcileInterval:             time.Duration(getInt("RECONCILE_INTERVAL", 1800)) * time.Second,
		PrintTableInterval:            time.Duration(getInt("PRINT_TABLE_INTERVAL", 60)) * time.Second,
		IncompleteRadiusPacketTimeout: time.Duration(getInt("INCOMPLETE_RADIUS_PACKET_TIMEOUT", 30)) * time.Second,
	}
	if cfg.MikroTikPassword == "" {
		return cfg, fmt.Errorf("MIKROTIK_PASSWORD is required")
	}
	if cfg.AdGuardPassword == "" {
		return cfg, fmt.Errorf("ADGUARD_PASSWORD is required")
	}
	if cfg.NxFilterEnabled {
		if cfg.NxFilterHost == "" {
			return cfg, fmt.Errorf("NXFILTER_HOST is required when NXFILTER_ENABLED=true")
		}
		if cfg.NxFilterSharedSecret == "" {
			return cfg, fmt.Errorf("NXFILTER_SHARED_SECRET is required when NXFILTER_ENABLED=true")
		}
	}
	return cfg, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func getInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
func getBool(k string, def bool) bool {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(k))); v != "" {
		return v == "1" || v == "true" || v == "yes" || v == "on"
	}
	return def
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	return s.Err()
}

func getCSV(k, def string) []string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		v = def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if x := strings.TrimSpace(part); x != "" {
			out = append(out, x)
		}
	}
	return out
}
