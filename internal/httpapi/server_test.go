package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/state"
	appstatus "git.tai.pe/homelab/mikrotik-adguard-identity/internal/status"
)

func TestStatusIsAggregateOnly(t *testing.T) {
	st := state.New()
	st.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s1", Username: "Alice", MAC: "AA:BB:CC:DD:EE:01", NASID: "NGFW"})
	st.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s2", Username: "Alice", MAC: "AA:BB:CC:DD:EE:02", NASID: "NGFW"})
	st.ApplyDHCP(models.DHCPEvent{MAC: "AA:BB:CC:DD:EE:01", IP: "192.168.2.10"})

	tracker := appstatus.New()
	now := time.Now()
	tracker.RouterOSSuccess(now)
	tracker.AdGuardSuccess(now)
	srv := &Server{State: st, Status: tracker}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	srv.apiStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, secret := range []string{"alice", "AA:BB:CC:DD:EE:01", "192.168.2.10", "NGFW", "s1"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(secret)) {
			t.Fatalf("response leaked %q: %s", secret, body)
		}
	}

	var got statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Radius.Users != 1 || got.Radius.Sessions != 2 || got.Radius.NAS != 1 {
		t.Fatalf("unexpected radius stats: %+v", got.Radius)
	}
	if got.Identity.MappedIPs != 1 || got.Identity.UnmappedSessions != 1 || got.Identity.DHCPLeases != 1 {
		t.Fatalf("unexpected identity stats: %+v", got.Identity)
	}
}

func TestHealthDegradedOnComponentError(t *testing.T) {
	tracker := appstatus.New()
	tracker.RouterOSSuccess(time.Now())
	tracker.AdGuardError(time.Now(), errors.New("dial tcp 192.168.0.103:80: timeout"))
	srv := &Server{State: state.New(), Status: tracker}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.health(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want 503", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "192.168.0.103") {
		t.Fatalf("health leaked infrastructure address: %s", rr.Body.String())
	}
}

func TestKidControlRequiresBearerToken(t *testing.T) {
	st := state.New()
	st.ApplyRadius(models.RadiusEvent{Type: models.RadiusStart, SessionID: "s1", Username: "Alice", MAC: "AA:BB:CC:DD:EE:01", NASID: "NGFW"})
	srv := &Server{State: st, Status: appstatus.New(), KidControlToken: "secret-token"}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/kid-control", nil)
	srv.apiKidControl(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/kid-control", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	srv.apiKidControl(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"user":"alice"`) || !strings.Contains(body, `"mac":"AA:BB:CC:DD:EE:01"`) {
		t.Fatalf("unexpected kid-control response: %s", body)
	}
}

func TestKidControlDisabledWithoutToken(t *testing.T) {
	srv := &Server{State: state.New(), Status: appstatus.New()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/kid-control", nil)
	srv.apiKidControl(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404", rr.Code)
	}
}
