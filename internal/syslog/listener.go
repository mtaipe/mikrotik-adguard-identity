package syslog

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/adguard"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/dhcp"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/nxfilter"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/radius"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/routeros"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
)

type Listener struct {
	Address           string
	Port              int
	State             *state.State
	Radius            *radius.Parser
	Reconciler        routeros.Reconciler
	AdGuard           *adguard.Client
	NxFilter          *nxfilter.Client
	ReconcileInterval time.Duration
	PrintInterval     time.Duration
	SyncDebounce      time.Duration
	dirty             atomic.Bool
}

func (l *Listener) markDirty() { l.dirty.Store(true) }

func (l *Listener) Run(ctx context.Context) error {
	addr := &net.UDPAddr{IP: net.ParseIP(l.Address), Port: l.Port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("syslog listener: udp://%s:%d", l.Address, l.Port)

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
		raw := string(buf[:n])
		source := remote.IP.String()
		if ev, ok := l.Radius.Feed(raw, source); ok {
			if l.State.ApplyRadius(ev) {
				log.Printf("RADIUS %s user=%s mac=%s nas=%s nas_ip=%s port=%s session=%s", ev.Type, ev.Username, ev.MAC, ev.NASID, ev.NASIP, ev.NASPortID, ev.SessionID)
				l.markDirty()
			}
			continue
		}
		if ev, ok := dhcp.Parse(raw); ok {
			if l.State.ApplyDHCP(ev) {
				user, _, _ := l.State.IdentityForMAC(ev.MAC)
				log.Printf("DHCP mac=%s ip=%s user=%s", ev.MAC, ev.IP, user)
				l.markDirty()
			}
		}
	}
}

func (l *Listener) timers(ctx context.Context) {
	reconcile := time.NewTicker(l.ReconcileInterval)
	defer reconcile.Stop()
	printT := time.NewTicker(l.PrintInterval)
	defer printT.Stop()
	cleanup := time.NewTicker(10 * time.Second)
	defer cleanup.Stop()
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
			if l.Reconciler.Run(false) {
				l.markDirty()
			}
		case <-printT.C:
			log.Print("\n" + l.State.Table())
		case <-cleanup.C:
			l.Radius.Cleanup()
		case <-syncT.C:
			if l.dirty.Swap(false) {
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
				if failed {
					l.markDirty()
				}
			}
		case <-nxRefreshC:
			if l.NxFilter != nil {
				if err := l.NxFilter.Sync(ctx, true); err != nil {
					log.Printf("nxFilter refresh failed: %v", err)
				}
			}
		}
	}
}

func (l *Listener) String() string { return fmt.Sprintf("%s:%d", l.Address, l.Port) }
