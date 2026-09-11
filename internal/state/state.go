package state

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/util"
)

type State struct {
	mu       sync.RWMutex
	sessions map[string]models.RadiusSession
	bindings map[string]models.DHCPBinding // MAC -> binding
}

func New() *State {
	return &State{sessions: map[string]models.RadiusSession{}, bindings: map[string]models.DHCPBinding{}}
}

func sessionKey(nasID, nasIP, sid string) string {
	nas := nasID
	if nas == "" {
		nas = nasIP
	}
	if nas == "" {
		nas = "unknown"
	}
	return nas + "|" + sid
}

func (s *State) ApplyRadius(ev models.RadiusEvent) bool {
	k := sessionKey(ev.NASID, ev.NASIP, ev.SessionID)
	username := util.NormalizeUsername(ev.Username)
	mac := util.NormalizeMAC(ev.MAC)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.Type == models.RadiusStop {
		if _, ok := s.sessions[k]; ok {
			delete(s.sessions, k)
			return true
		}
		return false
	}
	n := models.RadiusSession{
		Key: k, SessionID: ev.SessionID, Username: username,
		MAC: mac, NASID: ev.NASID, NASIP: ev.NASIP,
		NASPortID: ev.NASPortID, UpdatedAt: now,
	}
	old, ok := s.sessions[k]
	s.sessions[k] = n
	return !ok || old.Username != n.Username || old.MAC != n.MAC || old.NASID != n.NASID || old.NASIP != n.NASIP || old.NASPortID != n.NASPortID
}

func (s *State) ApplyDHCP(ev models.DHCPEvent) bool {
	mac := util.NormalizeMAC(ev.MAC)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.bindings[mac]
	s.bindings[mac] = models.DHCPBinding{IP: ev.IP, MAC: mac, UpdatedAt: now}
	return !ok || old.IP != ev.IP
}

func (s *State) ReconcileSessions(items []models.RadiusSession) bool {
	next := make(map[string]models.RadiusSession, len(items))
	for _, x := range items {
		x.Username = util.NormalizeUsername(x.Username)
		x.MAC = util.NormalizeMAC(x.MAC)
		x.Key = sessionKey(x.NASID, x.NASIP, x.SessionID)
		x.UpdatedAt = time.Now()
		next[x.Key] = x
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := len(next) != len(s.sessions)
	if !changed {
		for k, n := range next {
			o, ok := s.sessions[k]
			if !ok || o.Username != n.Username || o.MAC != n.MAC || o.NASID != n.NASID || o.NASIP != n.NASIP || o.NASPortID != n.NASPortID {
				changed = true
				break
			}
		}
	}
	s.sessions = next
	return changed
}

func (s *State) ReconcileDHCP(items []models.DHCPBinding) bool {
	next := make(map[string]models.DHCPBinding, len(items))
	for _, x := range items {
		x.MAC = util.NormalizeMAC(x.MAC)
		x.UpdatedAt = time.Now()
		next[x.MAC] = x
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := len(next) != len(s.bindings)
	if !changed {
		for k, n := range next {
			if o, ok := s.bindings[k]; !ok || o.IP != n.IP {
				changed = true
				break
			}
		}
	}
	s.bindings = next
	return changed
}

type Stats struct {
	Users            int
	Sessions         int
	NAS              int
	MappedIPs        int
	UnmappedSessions int
	DHCPLeases       int
}

func (s *State) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := map[string]struct{}{}
	nas := map[string]struct{}{}
	mappedIPs := map[string]struct{}{}
	unmapped := 0

	for _, sess := range s.sessions {
		users[sess.Username] = struct{}{}
		n := sess.NASID
		if n == "" {
			n = sess.NASIP
		}
		if n != "" {
			nas[n] = struct{}{}
		}
		if b, ok := s.bindings[sess.MAC]; ok && b.IP != "" {
			mappedIPs[b.IP] = struct{}{}
		} else {
			unmapped++
		}
	}

	return Stats{
		Users:            len(users),
		Sessions:         len(s.sessions),
		NAS:              len(nas),
		MappedIPs:        len(mappedIPs),
		UnmappedSessions: unmapped,
		DHCPLeases:       len(s.bindings),
	}
}

func (s *State) UserIPs() map[string]map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]map[string]struct{}{}
	for _, sess := range s.sessions {
		if _, ok := out[sess.Username]; !ok {
			out[sess.Username] = map[string]struct{}{}
		}
		if b, ok := s.bindings[sess.MAC]; ok && b.IP != "" {
			out[sess.Username][b.IP] = struct{}{}
		}
	}
	return out
}

func (s *State) IdentityForMAC(mac string) (string, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mac = util.NormalizeMAC(mac)
	var user string
	for _, sess := range s.sessions {
		if sess.MAC == mac {
			user = sess.Username
			break
		}
	}
	b, ok := s.bindings[mac]
	return user, b.IP, user != "" || ok
}

type KidControlDevice struct {
	User string `json:"user"`
	MAC  string `json:"mac"`
}

// KidControlDevices returns the current RADIUS-derived user-to-MAC ownership
// map. A MAC with multiple active sessions is assigned to the most recently
// updated session. Kid Control itself remains the persistent device store.
func (s *State) KidControlDevices() []KidControlDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type owner struct {
		user      string
		updatedAt time.Time
	}
	owners := map[string]owner{}
	for _, sess := range s.sessions {
		if sess.Username == "" || sess.MAC == "" {
			continue
		}
		old, ok := owners[sess.MAC]
		if !ok || sess.UpdatedAt.After(old.updatedAt) {
			owners[sess.MAC] = owner{user: sess.Username, updatedAt: sess.UpdatedAt}
		}
	}

	out := make([]KidControlDevice, 0, len(owners))
	for mac, o := range owners {
		out = append(out, KidControlDevice{User: o.user, MAC: mac})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].User == out[j].User {
			return out[i].MAC < out[j].MAC
		}
		return out[i].User < out[j].User
	})
	return out
}

type NxFilterIdentity struct {
	User string
	MAC  string
	IP   string
}

// NxFilterIdentities returns the current resolved RADIUS user -> MAC -> IPv4
// mappings. Only active RADIUS sessions with a current DHCP binding are
// returned. Duplicate user/MAC/IP tuples are collapsed.
func (s *State) NxFilterIdentities() []NxFilterIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[string]struct{}{}
	out := []NxFilterIdentity{}
	for _, sess := range s.sessions {
		if sess.Username == "" || sess.MAC == "" {
			continue
		}
		b, ok := s.bindings[sess.MAC]
		if !ok || b.IP == "" {
			continue
		}
		k := sess.Username + "|" + sess.MAC + "|" + b.IP
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, NxFilterIdentity{User: sess.Username, MAC: sess.MAC, IP: b.IP})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].User != out[j].User {
			return out[i].User < out[j].User
		}
		if out[i].MAC != out[j].MAC {
			return out[i].MAC < out[j].MAC
		}
		return out[i].IP < out[j].IP
	})
	return out
}

func (s *State) Table() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type row struct{ user, mac, ip string }
	rows := []row{}
	for _, sess := range s.sessions {
		ip := ""
		if b, ok := s.bindings[sess.MAC]; ok {
			ip = b.IP
		}
		rows = append(rows, row{sess.Username, sess.MAC, ip})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].user == rows[j].user {
			return rows[i].mac < rows[j].mac
		}
		return rows[i].user < rows[j].user
	})
	out := fmt.Sprintf("active identities: %d\n", len(rows))
	for _, r := range rows {
		out += fmt.Sprintf("  %-24s %-17s %s\n", r.user, r.mac, r.ip)
	}
	return out
}
