package nxfilter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
)

type Identity struct {
	User string
	MAC  string
	IP   string
}

type Client struct {
	Enabled         bool
	Host            string
	Port            int
	Secret          string
	NASIdentifier   string
	Timeout         time.Duration
	RefreshInterval time.Duration
	State           *state.State
	Status          *appstatus.Tracker

	mu       sync.Mutex
	current  map[string]Identity // user|mac|ip -> mapping last announced to nxFilter
	sequence atomic.Uint32
}

func New(enabled bool, host string, port int, secret, nasIdentifier string, timeout, refresh time.Duration, st *state.State, tracker *appstatus.Tracker) *Client {
	return &Client{
		Enabled: enabled, Host: host, Port: port, Secret: secret, NASIdentifier: nasIdentifier,
		Timeout: timeout, RefreshInterval: refresh, State: st, Status: tracker, current: map[string]Identity{},
	}
}

func (c *Client) Sync(ctx context.Context, forceRefresh bool) error {
	if !c.Enabled {
		if c.Status != nil {
			c.Status.NxFilterDisabled()
		}
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	var syncErr error
	defer func() {
		if c.Status == nil {
			return
		}
		if syncErr != nil {
			c.Status.NxFilterError(time.Now(), syncErr)
		} else {
			c.Status.NxFilterSuccess(time.Now())
		}
	}()

	desiredList := c.State.NxFilterIdentities()
	desired := make(map[string]Identity, len(desiredList))
	ownersByIP := map[string]map[string]struct{}{}
	for _, x := range desiredList {
		i := Identity{User: x.User, MAC: x.MAC, IP: x.IP}
		desired[identityKey(i)] = i
		if ownersByIP[i.IP] == nil {
			ownersByIP[i.IP] = map[string]struct{}{}
		}
		ownersByIP[i.IP][i.User] = struct{}{}
	}
	for ip, owners := range ownersByIP {
		if len(owners) <= 1 {
			continue
		}
		users := make([]string, 0, len(owners))
		for user := range owners {
			users = append(users, user)
		}
		sort.Strings(users)
		log.Printf("nxFilter identity conflict: IP %s is associated with users %v; leaving it unassigned", ip, users)
		for k, i := range desired {
			if i.IP == ip {
				delete(desired, k)
			}
		}
	}

	// Stop mappings that no longer exist before starting replacements. This is
	// important when an IP moves between users or a device receives a new IP.
	staleKeys := make([]string, 0)
	for k := range c.current {
		if _, ok := desired[k]; !ok {
			staleKeys = append(staleKeys, k)
		}
	}
	sort.Strings(staleKeys)
	for _, k := range staleKeys {
		i := c.current[k]
		if err := c.send(ctx, i, acctStatusStop); err != nil {
			syncErr = fmt.Errorf("nxFilter stop user=%s ip=%s: %w", i.User, i.IP, err)
			return syncErr
		}
		log.Printf("nxFilter Stop user=%s mac=%s ip=%s", i.User, i.MAC, i.IP)
		delete(c.current, k)
	}

	keys := make([]string, 0, len(desired))
	for k := range desired {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		i := desired[k]
		if _, ok := c.current[k]; !ok {
			if err := c.send(ctx, i, acctStatusStart); err != nil {
				syncErr = fmt.Errorf("nxFilter start user=%s ip=%s: %w", i.User, i.IP, err)
				return syncErr
			}
			log.Printf("nxFilter Start user=%s mac=%s ip=%s", i.User, i.MAC, i.IP)
			c.current[k] = i
			continue
		}
		if forceRefresh {
			if err := c.send(ctx, i, acctStatusInterim); err != nil {
				syncErr = fmt.Errorf("nxFilter interim user=%s ip=%s: %w", i.User, i.IP, err)
				return syncErr
			}
			log.Printf("nxFilter Interim-Update user=%s mac=%s ip=%s", i.User, i.MAC, i.IP)
		}
	}
	return nil
}

func (c *Client) StopAll(ctx context.Context) {
	if !c.Enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]string, 0, len(c.current))
	for k := range c.current {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		i := c.current[k]
		if err := c.send(ctx, i, acctStatusStop); err != nil {
			log.Printf("nxFilter shutdown Stop failed user=%s ip=%s: %v", i.User, i.IP, err)
			continue
		}
		delete(c.current, k)
	}
}

func (c *Client) send(ctx context.Context, i Identity, status uint32) error {
	ipAttr, err := ipv4Attr(attrFramedIPAddress, i.IP)
	if err != nil {
		return err
	}
	attrs := []radiusAttribute{
		textAttr(attrUserName, i.User),
		ipAttr,
		textAttr(attrCallingStationID, i.MAC),
		textAttr(attrNASIdentifier, c.NASIdentifier),
		uint32Attr(attrAcctStatusType, status),
		textAttr(attrAcctSessionID, sessionID(i)),
		uint32Attr(attrAcctAuthentic, acctAuthenticRADIUS),
	}
	id := byte(c.sequence.Add(1))
	packet, err := encodeAccountingRequest(id, c.Secret, attrs)
	if err != nil {
		return err
	}

	d := net.Dialer{Timeout: c.Timeout}
	conn, err := d.DialContext(ctx, "udp", net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(c.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write(packet); err != nil {
		return err
	}
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		return err
	}
	return validateAccountingResponse(response[:n], packet, c.Secret)
}

func identityKey(i Identity) string { return i.User + "|" + i.MAC + "|" + i.IP }

func sessionID(i Identity) string {
	sum := sha256.Sum256([]byte(identityKey(i)))
	return "is-" + hex.EncodeToString(sum[:8])
}
