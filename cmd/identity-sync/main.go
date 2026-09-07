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
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/radius"
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
	st := state.New()
	tracker := appstatus.New()
	rc := routeros.NewClient(cfg.MikroTikHost, cfg.MikroTikPort, cfg.MikroTikUser, cfg.MikroTikPassword)
	reconciler := routeros.Reconciler{Client: rc, State: st, Status: tracker}
	ag := adguard.New(cfg.AdGuardURL, cfg.AdGuardUser, cfg.AdGuardPassword, cfg.AdGuardVerifyTLS, cfg.AdGuardTimeout, cfg.AdGuardRetryInterval, st, tracker)

	reconciler.Run(true)
	log.Print("\n" + st.Table())
	if err := ag.Sync(true); err != nil {
		log.Printf("initial AdGuard sync failed: %v", err)
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

	listener := &appsyslog.Listener{Address: cfg.SyslogAddress, Port: cfg.SyslogPort, State: st, Radius: radius.New(cfg.IncompleteRadiusPacketTimeout), Reconciler: reconciler, AdGuard: ag, ReconcileInterval: cfg.ReconcileInterval, PrintInterval: cfg.PrintTableInterval, SyncDebounce: cfg.AdGuardSyncDebounce}
	if err := listener.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
