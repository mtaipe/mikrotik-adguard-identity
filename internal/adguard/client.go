package adguard

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/util"
)

type Client struct {
	BaseURL       string
	User          string
	Password      string
	HTTP          *http.Client
	State         *state.State
	Status        *appstatus.Tracker
	RetryInterval time.Duration

	mu          sync.Mutex
	lastFailure time.Time
}

type clientsResponse struct {
	Clients []map[string]any `json:"clients"`
}

func New(baseURL, user, password string, verifyTLS bool, timeout, retry time.Duration, st *state.State, tracker *appstatus.Tracker) *Client {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS}}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), User: user, Password: password, HTTP: &http.Client{Timeout: timeout, Transport: tr}, State: st, Status: tracker, RetryInterval: retry}
}

func (c *Client) Sync(force bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && !c.lastFailure.IsZero() && time.Since(c.lastFailure) < c.RetryInterval {
		return nil
	}
	if err := c.sync(); err != nil {
		c.lastFailure = time.Now()
		if c.Status != nil {
			c.Status.AdGuardError(time.Now(), err)
		}
		return err
	}
	c.lastFailure = time.Time{}
	if c.Status != nil {
		c.Status.AdGuardSuccess(time.Now())
	}
	return nil
}

func (c *Client) sync() error {
	clients, err := c.list()
	if err != nil {
		return err
	}
	currentUsers := c.State.UserIPs()
	existingByName := map[string]map[string]any{}
	managed := map[string]struct{}{}
	for _, cl := range clients {
		name, _ := cl["name"].(string)
		canon := util.NormalizeUsername(name)
		if canon == "" {
			continue
		}
		existingByName[canon] = cl
		for _, id := range idsOf(cl) {
			if strings.HasPrefix(id, "radius-") {
				managed[canon] = struct{}{}
				break
			}
		}
	}
	usernames := map[string]struct{}{}
	for u := range currentUsers {
		usernames[u] = struct{}{}
	}
	for u := range managed {
		usernames[u] = struct{}{}
	}

	desired := map[string][]string{}
	for u := range usernames {
		ids := []string{util.StableClientID(u)}
		// Preserve existing non-IP identifiers (for example an encrypted-DNS ClientID).
		if cl := existingByName[u]; cl != nil {
			for _, id := range idsOf(cl) {
				if !util.IsIPv4(id) && id != util.StableClientID(u) {
					ids = append(ids, id)
				}
			}
		}
		ips := make([]string, 0, len(currentUsers[u]))
		for ip := range currentUsers[u] {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		ids = append(ids, ips...)
		desired[u] = ids
	}

	// If state itself says one IP belongs to multiple users, do not assign it to either.
	desiredOwners := map[string][]string{}
	for user, ids := range desired {
		for _, id := range ids {
			if util.IsIPv4(id) {
				desiredOwners[id] = append(desiredOwners[id], user)
			}
		}
	}
	for ip, owners := range desiredOwners {
		if len(owners) <= 1 {
			continue
		}
		sort.Strings(owners)
		log.Printf("identity conflict: IP %s is simultaneously associated with users %v; leaving it unassigned", ip, owners)
		for _, user := range owners {
			filtered := desired[user][:0]
			for _, id := range desired[user] {
				if id != ip {
					filtered = append(filtered, id)
				}
			}
			desired[user] = append([]string(nil), filtered...)
		}
	}

	// Protect manually managed clients that own a desired IP for a different user.
	ownerByIP := map[string]string{}
	for canon, cl := range existingByName {
		for _, id := range idsOf(cl) {
			if util.IsIPv4(id) {
				ownerByIP[id] = canon
			}
		}
	}
	for user, ids := range desired {
		filtered := ids[:0]
		for _, id := range ids {
			if !util.IsIPv4(id) {
				filtered = append(filtered, id)
				continue
			}
			owner := ownerByIP[id]
			if owner != "" && owner != user {
				if _, ok := managed[owner]; !ok {
					log.Printf("AdGuard conflict: IP %s belongs to unmanaged client %q; skipping assignment to %q", id, owner, user)
					continue
				}
			}
			filtered = append(filtered, id)
		}
		desired[user] = append([]string(nil), filtered...)
	}

	// Phase 1: remove stale IP ownership from managed/matched users, but do not add new IPs yet.
	for user, ids := range desired {
		cl := existingByName[user]
		if cl == nil {
			continue
		}
		current := idsOf(cl)
		currentSet := toSet(current)
		desiredSet := toSet(ids)
		phase1 := []string{}
		for _, id := range ids {
			if !util.IsIPv4(id) {
				phase1 = append(phase1, id)
			}
		}
		changed := false
		for id := range currentSet {
			if util.IsIPv4(id) && desiredSet[id] {
				phase1 = append(phase1, id)
			}
			if util.IsIPv4(id) && !desiredSet[id] {
				changed = true
			}
		}
		sort.Strings(phase1[1:])
		if changed || !sameStringSet(current, phase1) {
			if err := c.update(cl, phase1); err != nil {
				return fmt.Errorf("release stale IDs for %s: %w", user, err)
			}
			log.Printf("AdGuard phase 1: %s ids=%v", displayName(cl, user), phase1)
		}
	}

	// Re-read after releases to avoid duplicate-IP races.
	clients, err = c.list()
	if err != nil {
		return err
	}
	existingByName = map[string]map[string]any{}
	for _, cl := range clients {
		if name, _ := cl["name"].(string); name != "" {
			existingByName[util.NormalizeUsername(name)] = cl
		}
	}

	// Phase 2: assign complete desired state. Existing policy fields are preserved.
	for user, ids := range desired {
		cl := existingByName[user]
		if cl == nil {
			if err := c.add(user, ids); err != nil {
				return fmt.Errorf("add client %s: %w", user, err)
			}
			log.Printf("AdGuard add: %s ids=%v", user, ids)
			continue
		}
		if sameStringSet(idsOf(cl), ids) {
			continue
		}
		if err := c.update(cl, ids); err != nil {
			return fmt.Errorf("update client %s: %w", user, err)
		}
		log.Printf("AdGuard update: %s ids=%v", displayName(cl, user), ids)
	}
	return nil
}

func (c *Client) list() ([]map[string]any, error) {
	var out clientsResponse
	if err := c.request(http.MethodGet, "/control/clients", nil, &out); err != nil {
		return nil, err
	}
	return out.Clients, nil
}
func (c *Client) add(name string, ids []string) error {
	data := map[string]any{"name": name, "ids": ids, "use_global_settings": true, "use_global_blocked_services": true, "filtering_enabled": true, "parental_enabled": false, "safebrowsing_enabled": false, "ignore_querylog": false, "ignore_statistics": false}
	return c.request(http.MethodPost, "/control/clients/add", data, nil)
}
func (c *Client) update(existing map[string]any, ids []string) error {
	name, _ := existing["name"].(string)
	data := cloneAllowed(existing)
	data["name"] = name
	data["ids"] = ids
	payload := map[string]any{"name": name, "data": data}
	return c.request(http.MethodPost, "/control/clients/update", payload, nil)
}

var allowed = map[string]struct{}{
	"name": {}, "ids": {}, "use_global_settings": {}, "filtering_enabled": {}, "parental_enabled": {}, "safebrowsing_enabled": {}, "safe_search": {}, "use_global_blocked_services": {}, "blocked_services_schedule": {}, "blocked_services": {}, "upstreams": {}, "tags": {}, "ignore_querylog": {}, "ignore_statistics": {}, "upstreams_cache_enabled": {}, "upstreams_cache_size": {},
}

func cloneAllowed(src map[string]any) map[string]any {
	dst := map[string]any{}
	for k, v := range src {
		if _, ok := allowed[k]; ok {
			dst[k] = v
		}
	}
	return dst
}
func idsOf(cl map[string]any) []string {
	v := cl["ids"]
	a, ok := v.([]any)
	if ok {
		out := make([]string, 0, len(a))
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := v.([]string); ok {
		return append([]string(nil), s...)
	}
	return nil
}
func toSet(a []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	return m
}
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
func displayName(cl map[string]any, fallback string) string {
	if s, ok := cl["name"].(string); ok && s != "" {
		return s
	}
	return fallback
}

func (c *Client) request(method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, r)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return err
		}
	}
	return nil
}
