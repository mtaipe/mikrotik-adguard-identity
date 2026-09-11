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
	HTTPAddress          string
	KidControlAPIToken   string

	NxFilterEnabled         bool
	NxFilterHost            string
	NxFilterAccountingPort  int
	NxFilterSharedSecret    string
	NxFilterNASIdentifier   string
	NxFilterTimeout         time.Duration
	NxFilterRefreshInterval time.Duration

	LogLevel string

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
		LogLevel:                      get("LOG_LEVEL", "info"),
		ReconcileInterval:             time.Duration(getInt("RECONCILE_INTERVAL", 1800)) * time.Second,
		PrintTableInterval:            time.Duration(getInt("PRINT_TABLE_INTERVAL", 0)) * time.Second,
		IncompleteRadiusPacketTimeout: time.Duration(getInt("INCOMPLETE_RADIUS_PACKET_TIMEOUT", 30)) * time.Second,
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.MikroTikPassword == "" {
		return fmt.Errorf("MIKROTIK_PASSWORD is required")
	}
	if cfg.AdGuardPassword == "" {
		return fmt.Errorf("ADGUARD_PASSWORD is required")
	}
	if cfg.NxFilterEnabled {
		if cfg.NxFilterHost == "" {
			return fmt.Errorf("NXFILTER_HOST is required when NXFILTER_ENABLED=true")
		}
		if cfg.NxFilterSharedSecret == "" {
			return fmt.Errorf("NXFILTER_SHARED_SECRET is required when NXFILTER_ENABLED=true")
		}
	}

	for _, item := range []struct {
		name string
		port int
	}{
		{"MIKROTIK_PORT", cfg.MikroTikPort},
		{"SYSLOG_LISTEN_PORT", cfg.SyslogPort},
		{"NXFILTER_ACCOUNTING_PORT", cfg.NxFilterAccountingPort},
	} {
		if item.port < 1 || item.port > 65535 {
			return fmt.Errorf("%s must be between 1 and 65535", item.name)
		}
	}

	for _, item := range []struct {
		name  string
		value time.Duration
	}{
		{"ADGUARD_TIMEOUT", cfg.AdGuardTimeout},
		{"ADGUARD_RETRY_INTERVAL", cfg.AdGuardRetryInterval},
		{"ADGUARD_SYNC_DEBOUNCE", cfg.AdGuardSyncDebounce},
		{"RECONCILE_INTERVAL", cfg.ReconcileInterval},
		{"INCOMPLETE_RADIUS_PACKET_TIMEOUT", cfg.IncompleteRadiusPacketTimeout},
	} {
		if item.value <= 0 {
			return fmt.Errorf("%s must be greater than zero", item.name)
		}
	}

	if cfg.PrintTableInterval < 0 {
		return fmt.Errorf("PRINT_TABLE_INTERVAL cannot be negative")
	}
	if cfg.NxFilterEnabled {
		if cfg.NxFilterTimeout <= 0 {
			return fmt.Errorf("NXFILTER_TIMEOUT must be greater than zero")
		}
		if cfg.NxFilterRefreshInterval < 0 {
			return fmt.Errorf("NXFILTER_REFRESH_INTERVAL cannot be negative")
		}
	}
	if cfg.KidControlAPIToken != "" && len(cfg.KidControlAPIToken) < 32 {
		return fmt.Errorf("KID_CONTROL_API_TOKEN must be at least 32 characters when enabled")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "info", "debug":
	default:
		return fmt.Errorf("LOG_LEVEL must be info or debug")
	}
	return nil
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
