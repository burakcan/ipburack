package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/burakcan/ipburack/internal/geodb"
)

// mockGeoLookup implements GeoLookup for testing
type mockGeoLookup struct {
	result *geodb.LookupResult
	err    error
}

func (m *mockGeoLookup) Lookup(ip string, useCity bool) (*geodb.LookupResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestHealth(t *testing.T) {
	h := New(&mockGeoLookup{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", resp.Status)
	}

	if resp.Uptime == "" {
		t.Error("expected uptime to be set")
	}
}

func TestLookupIP_Success(t *testing.T) {
	mock := &mockGeoLookup{
		result: &geodb.LookupResult{CountryCode: "US"},
	}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup/8.8.8.8", nil)
	w := httptest.NewRecorder()

	h.LookupIP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp LookupResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.CountryCode != "US" {
		t.Errorf("expected country code 'US', got %q", resp.CountryCode)
	}
}

func TestLookupIP_WithPostalCode(t *testing.T) {
	mock := &mockGeoLookup{
		result: &geodb.LookupResult{CountryCode: "US", PostalCode: "10001"},
	}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup/8.8.8.8?pc=true", nil)
	w := httptest.NewRecorder()

	h.LookupIP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp LookupResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.CountryCode != "US" {
		t.Errorf("expected country code 'US', got %q", resp.CountryCode)
	}

	if resp.PostalCode != "10001" {
		t.Errorf("expected postal code '10001', got %q", resp.PostalCode)
	}
}

func TestLookupIP_MissingIP(t *testing.T) {
	h := New(&mockGeoLookup{})

	req := httptest.NewRequest(http.MethodGet, "/lookup/", nil)
	w := httptest.NewRecorder()

	h.LookupIP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLookupIP_InvalidIP(t *testing.T) {
	mock := &mockGeoLookup{err: geodb.ErrInvalidIP}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup/invalid", nil)
	w := httptest.NewRecorder()

	h.LookupIP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLookupIP_NotFound(t *testing.T) {
	mock := &mockGeoLookup{err: geodb.ErrIPNotFound}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup/192.168.1.1", nil)
	w := httptest.NewRecorder()

	h.LookupIP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestLookupSelf_XForwardedFor(t *testing.T) {
	mock := &mockGeoLookup{
		result: &geodb.LookupResult{CountryCode: "DE"},
	}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
	w := httptest.NewRecorder()

	h.LookupSelf(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestLookupSelf_XRealIP(t *testing.T) {
	mock := &mockGeoLookup{
		result: &geodb.LookupResult{CountryCode: "FR"},
	}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup", nil)
	req.Header.Set("X-Real-IP", "203.0.113.50")
	w := httptest.NewRecorder()

	h.LookupSelf(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestLookupSelf_RemoteAddr(t *testing.T) {
	mock := &mockGeoLookup{
		result: &geodb.LookupResult{CountryCode: "GB"},
	}
	h := New(mock)

	req := httptest.NewRequest(http.MethodGet, "/lookup", nil)
	req.RemoteAddr = "203.0.113.100:12345"
	w := httptest.NewRecorder()

	h.LookupSelf(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xRealIP    string
		remoteAddr string
		want       string
	}{
		{
			name: "X-Forwarded-For single",
			xff:  "203.0.113.1",
			want: "203.0.113.1",
		},
		{
			name: "X-Forwarded-For multiple",
			xff:  "203.0.113.1, 198.51.100.1, 192.0.2.1",
			want: "203.0.113.1",
		},
		{
			name: "X-Forwarded-For with spaces",
			xff:  "  203.0.113.1  ",
			want: "203.0.113.1",
		},
		{
			name:    "X-Real-IP",
			xRealIP: "203.0.113.50",
			want:    "203.0.113.50",
		},
		{
			name:       "RemoteAddr with port",
			remoteAddr: "203.0.113.100:12345",
			want:       "203.0.113.100",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "203.0.113.100",
			want:       "203.0.113.100",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			xff:        "203.0.113.1",
			xRealIP:    "203.0.113.50",
			remoteAddr: "203.0.113.100:12345",
			want:       "203.0.113.1",
		},
		{
			name:       "X-Real-IP takes precedence over RemoteAddr",
			xRealIP:    "203.0.113.50",
			remoteAddr: "203.0.113.100:12345",
			want:       "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
