package logging

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
)

var debugEnabled atomic.Bool

func Configure(level string) error {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		debugEnabled.Store(false)
	case "debug":
		debugEnabled.Store(true)
	default:
		return fmt.Errorf("LOG_LEVEL must be info or debug")
	}
	return nil
}

func Debugf(format string, args ...any) {
	if debugEnabled.Load() {
		log.Printf("DEBUG "+format, args...)
	}
}
