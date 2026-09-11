package routeros

import (
	"log"
	"time"

	applog "git.tai.pe/homelab/mikrotik-adguard-identity/internal/logging"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
)

type Reconciler struct {
	Client Client
	State  *state.State
	Status *appstatus.Tracker
}

// Run fetches both authoritative identity inputs from RouterOS before mutating
// state. This is intentionally atomic from a trust perspective: if either the
// active RADIUS sessions or bound DHCP leases cannot be read, no externally
// visible identity state is changed and callers must not publish a sync.
func (r Reconciler) Run(startup bool) (changed bool, verified bool) {
	sessions, err := r.Client.ActiveSessions()
	if err != nil {
		log.Printf("RouterOS active-session reconcile failed: %v", err)
		if r.Status != nil {
			r.Status.RouterOSError(time.Now(), err)
		}
		return false, false
	}

	leases, err := r.Client.BoundLeases()
	if err != nil {
		log.Printf("RouterOS DHCP reconcile failed: %v", err)
		if r.Status != nil {
			r.Status.RouterOSError(time.Now(), err)
		}
		return false, false
	}

	if r.State.ReconcileSessions(sessions) {
		changed = true
	}
	if r.State.ReconcileDHCP(leases) {
		changed = true
	}
	if startup || changed {
		log.Printf("RouterOS reconcile: %d active RADIUS sessions, %d bound DHCP leases", len(sessions), len(leases))
	} else {
		applog.Debugf("RouterOS reconcile unchanged: %d active RADIUS sessions, %d bound DHCP leases", len(sessions), len(leases))
	}

	if r.Status != nil {
		r.Status.RouterOSSuccess(time.Now())
	}
	return changed, true
}

func NewClient(host string, port int, user, password string) Client {
	return Client{Host: host, Port: port, User: user, Password: password, Timeout: 10 * time.Second}
}
