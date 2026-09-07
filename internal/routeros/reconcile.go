package routeros

import (
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
	"log"
	"time"
)

type Reconciler struct {
	Client Client
	State  *state.State
	Status *appstatus.Tracker
}

func (r Reconciler) Run(startup bool) bool {
	changed := false
	var reconcileErr error
	sessions, err := r.Client.ActiveSessions()
	if err != nil {
		reconcileErr = err
		log.Printf("RouterOS active-session reconcile failed: %v", err)
	} else {
		if r.State.ReconcileSessions(sessions) {
			changed = true
		}
		log.Printf("RouterOS reconcile: %d active RADIUS sessions", len(sessions))
	}
	leases, err := r.Client.BoundLeases()
	if err != nil {
		if reconcileErr == nil {
			reconcileErr = err
		}
		log.Printf("RouterOS DHCP reconcile failed: %v", err)
	} else {
		if r.State.ReconcileDHCP(leases) {
			changed = true
		}
		log.Printf("RouterOS reconcile: %d bound DHCP leases", len(leases))
	}
	if r.Status != nil {
		if reconcileErr != nil {
			r.Status.RouterOSError(time.Now(), reconcileErr)
		} else {
			r.Status.RouterOSSuccess(time.Now())
		}
	}
	_ = startup
	return changed
}

func NewClient(host string, port int, user, password string) Client {
	return Client{Host: host, Port: port, User: user, Password: password, Timeout: 10 * time.Second}
}
