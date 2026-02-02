package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/burakcan/ipburack/internal/geodb"
)

type GeoLookup interface {
	Lookup(ip string) (*geodb.LookupResult, error)
}

type Handlers struct {
	geo       GeoLookup
	startTime time.Time
}

func New(geo GeoLookup) *Handlers {
	return &Handlers{
		geo:       geo,
		startTime: time.Now(),
	}
}

type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status: "healthy",
		Uptime: time.Since(h.startTime).Round(time.Second).String(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) LookupIP(w http.ResponseWriter, r *http.Request) {
	// Extract IP from URL path: /lookup/{ip}
	path := strings.TrimPrefix(r.URL.Path, "/lookup/")
	if path == "" || path == r.URL.Path {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "IP address required"})
		return
	}

	h.doLookup(w, path)
}

func (h *Handlers) LookupSelf(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)
	if ip == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "could not determine client IP"})
		return
	}

	h.doLookup(w, ip)
}

type LookupResponse struct {
	CountryCode string `json:"country_code"`
}

func (h *Handlers) doLookup(w http.ResponseWriter, ip string) {
	result, err := h.geo.Lookup(ip)
	if err != nil {
		if errors.Is(err, geodb.ErrInvalidIP) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid IP address"})
			return
		}
		if errors.Is(err, geodb.ErrIPNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "IP not found in database"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "lookup failed"})
		return
	}

	writeJSON(w, http.StatusOK, LookupResponse{CountryCode: result.CountryCode})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
