package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/adguard"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/config"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/httpapi"
	applog "git.tai.pe/homelab/mikrotik-adguard-identity/internal/logging"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/nxfilter"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/routeros"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
	appsyslog "git.tai.pe/homelab/mikrotik-adguard-identity/internal/syslog"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := applog.Configure(cfg.LogLevel); err != nil {
		log.Fatal(err)
	}
	st := state.New()
	tracker := appstatus.New()
	rc := routeros.NewClient(cfg.MikroTikHost, cfg.MikroTikPort, cfg.MikroTikUser, cfg.MikroTikPassword)
	reconciler := routeros.Reconciler{Client: rc, State: st, Status: tracker}
	ag := adguard.New(cfg.AdGuardURL, cfg.AdGuardUser, cfg.AdGuardPassword, cfg.AdGuardVerifyTLS, cfg.AdGuardTimeout, cfg.AdGuardRetryInterval, st, tracker)
	nxf := nxfilter.New(cfg.NxFilterEnabled, cfg.NxFilterHost, cfg.NxFilterAccountingPort, cfg.NxFilterSharedSecret, cfg.NxFilterNASIdentifier, cfg.NxFilterTimeout, cfg.NxFilterRefreshInterval, st, tracker)

	_, verified := reconciler.Run(true)
	applog.Debugf("initial identity state:\n%s", st.Table())
	if verified {
		if err := ag.Sync(true); err != nil {
			log.Printf("initial AdGuard sync failed: %v", err)
		}
		if err := nxf.Sync(context.Background(), false); err != nil {
			log.Printf("initial nxFilter sync failed: %v", err)
		}
	} else {
		log.Printf("initial backend sync skipped: RouterOS state could not be fully verified")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	api := &httpapi.Server{Address: cfg.HTTPAddress, State: st, Status: tracker, KidControlToken: cfg.KidControlAPIToken}
	go func() {
		if err := api.Run(ctx); err != nil {
			log.Printf("HTTP status API failed: %v", err)
			stop()
		}
	}()

	listener := &appsyslog.Listener{Address: cfg.SyslogAddress, Port: cfg.SyslogPort, AllowedSources: cfg.SyslogAllowedSources, State: st, Reconciler: reconciler, AdGuard: ag, NxFilter: nxf, ReconcileInterval: cfg.ReconcileInterval, PrintInterval: cfg.PrintTableInterval, SyncDebounce: cfg.AdGuardSyncDebounce}
	if err := listener.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
