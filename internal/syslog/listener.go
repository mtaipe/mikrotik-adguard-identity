package syslog

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/adguard"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/dhcp"
	applog "git.tai.pe/homelab/mikrotik-adguard-identity/internal/logging"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/nxfilter"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/radius"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/routeros"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
)

type Listener struct {
	Address           string
	Port              int
	AllowedSources    []string
	State             *state.State
	Radius            *radius.Parser
	Reconciler        routeros.Reconciler
	AdGuard           *adguard.Client
	NxFilter          *nxfilter.Client
	ReconcileInterval time.Duration
	PrintInterval     time.Duration
	SyncDebounce      time.Duration
	dirty             atomic.Bool
	dropped           atomic.Uint64
}

func (l *Listener) markDirty() { l.dirty.Store(true) }

func (l *Listener) Run(ctx context.Context) error {
	addr := &net.UDPAddr{IP: net.ParseIP(l.Address), Port: l.Port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("syslog listener: udp://%s:%d allowed_sources=%s", l.Address, l.Port, strings.Join(l.AllowedSources, ","))

	go l.timers(ctx)
	buf := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, remote, err := conn.ReadFromUDP(buf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}
		if err != nil {
			return err
		}
		if !sourceAllowed(remote.IP, l.AllowedSources) {
			l.dropped.Add(1)
			continue
		}

		// SECURITY: syslog is deliberately trigger-only. UDP source addresses can
		// be spoofed, so parsed RADIUS/DHCP content is never applied to trusted
		// identity state. A relevant event only schedules a fresh read of the
		// authoritative RouterOS API state.
		raw := string(buf[:n])
		if _, ok := l.Radius.Feed(raw, remote.IP.String()); ok {
			l.markDirty()
			continue
		}
		if _, ok := dhcp.Parse(raw); ok {
			l.markDirty()
		}
	}
}

func sourceAllowed(ip net.IP, rules []string) bool {
	if ip == nil || len(rules) == 0 {
		return false
	}
	for _, raw := range rules {
		rule := strings.TrimSpace(raw)
		if rule == "" {
			continue
		}
		if strings.Contains(rule, "/") {
			_, network, err := net.ParseCIDR(rule)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if allowed := net.ParseIP(rule); allowed != nil && allowed.Equal(ip) {
			return true
		}
	}
	return false
}

func (l *Listener) syncBackends(ctx context.Context) bool {
	failed := false
	if err := l.AdGuard.Sync(false); err != nil {
		log.Printf("AdGuard sync failed: %v", err)
		failed = true
	}
	if l.NxFilter != nil {
		if err := l.NxFilter.Sync(ctx, false); err != nil {
			log.Printf("nxFilter sync failed: %v", err)
			failed = true
		}
	}
	return !failed
}

func (l *Listener) timers(ctx context.Context) {
	reconcile := time.NewTicker(l.ReconcileInterval)
	defer reconcile.Stop()
	var printT *time.Ticker
	var printC <-chan time.Time
	if l.PrintInterval > 0 {
		printT = time.NewTicker(l.PrintInterval)
		printC = printT.C
		defer printT.Stop()
	}
	cleanup := time.NewTicker(10 * time.Second)
	defer cleanup.Stop()
	dropReport := time.NewTicker(time.Minute)
	defer dropReport.Stop()
	syncT := time.NewTicker(l.SyncDebounce)
	defer syncT.Stop()
	var nxRefresh *time.Ticker
	var nxRefreshC <-chan time.Time
	if l.NxFilter != nil && l.NxFilter.Enabled && l.NxFilter.RefreshInterval > 0 {
		nxRefresh = time.NewTicker(l.NxFilter.RefreshInterval)
		nxRefreshC = nxRefresh.C
		defer nxRefresh.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcile.C:
			changed, verified := l.Reconciler.Run(false)
			if verified && changed && !l.syncBackends(ctx) {
				l.markDirty()
			}
		case <-printC:
			applog.Debugf("identity state:\n%s", l.State.Table())
		case <-dropReport.C:
			if n := l.dropped.Swap(0); n > 0 {
				log.Printf("syslog security: dropped %d datagrams from unauthorized sources since last report", n)
			}
		case <-cleanup.C:
			l.Radius.Cleanup()
		case <-syncT.C:
			if l.dirty.Swap(false) {
				// A syslog event is only a wake-up signal. Always re-read both
				// RouterOS sources and publish only after a complete verified read.
				changed, verified := l.Reconciler.Run(false)
				if !verified {
					l.markDirty()
					continue
				}
				if changed && !l.syncBackends(ctx) {
					l.markDirty()
				}
			}
		case <-nxRefreshC:
			if l.NxFilter == nil {
				continue
			}
			// Never extend nxFilter sessions from stale in-memory identity state.
			changed, verified := l.Reconciler.Run(false)
			if !verified {
				log.Printf("nxFilter refresh skipped: RouterOS state could not be fully verified")
				continue
			}
			if changed {
				if !l.syncBackends(ctx) {
					l.markDirty()
				}
				continue
			}
			if err := l.NxFilter.Sync(ctx, true); err != nil {
				log.Printf("nxFilter refresh failed: %v", err)
			}
		}
	}
}

func (l *Listener) String() string { return fmt.Sprintf("%s:%d", l.Address, l.Port) }
