package geodb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

var (
	ErrInvalidIP  = errors.New("invalid IP address")
	ErrIPNotFound = errors.New("IP not found in database")
)

// CountryRecord matches the structure in geolite2-geo-whois-asn-country MMDB
type CountryRecord struct {
	CountryCode string `maxminddb:"country_code"`
}

// CityRecord matches the structure in geolite2-city MMDB
type CityRecord struct {
	CountryCode string  `maxminddb:"country_code"`
	City        string  `maxminddb:"city"`
	PostCode    string  `maxminddb:"postcode"`
	Latitude    float64 `maxminddb:"latitude"`
	Longitude   float64 `maxminddb:"longitude"`
}

type LookupResult struct {
	CountryCode string `json:"country_code"`
	PostalCode  string `json:"postal_code,omitempty"`
}

type Logger interface {
	Info(message string, data map[string]any)
	Error(message string, data map[string]any)
}

type dbInstance struct {
	db   *maxminddb.Reader
	mu   sync.RWMutex
	path string
	url  string
}

type GeoDB struct {
	country        *dbInstance
	cityIPv4       *dbInstance
	cityIPv6       *dbInstance
	updateInterval time.Duration
	logger         Logger
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func New(countryPath, countryURL, cityIPv4Path, cityIPv4URL, cityIPv6Path, cityIPv6URL string, updateInterval time.Duration, logger Logger) *GeoDB {
	return &GeoDB{
		country:        &dbInstance{path: countryPath, url: countryURL},
		cityIPv4:       &dbInstance{path: cityIPv4Path, url: cityIPv4URL},
		cityIPv6:       &dbInstance{path: cityIPv6Path, url: cityIPv6URL},
		updateInterval: updateInterval,
		logger:         logger,
	}
}

func (g *GeoDB) Start(ctx context.Context) error {
	// Initialize all databases
	if err := g.initDB(g.country, "country"); err != nil {
		return err
	}
	if err := g.initDB(g.cityIPv4, "city-ipv4"); err != nil {
		return err
	}
	if err := g.initDB(g.cityIPv6, "city-ipv6"); err != nil {
		return err
	}

	// Start background update goroutine
	updateCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.wg.Add(1)
	go g.updateLoop(updateCtx)

	return nil
}

func (g *GeoDB) initDB(inst *dbInstance, name string) error {
	dir := filepath.Dir(inst.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if _, err := os.Stat(inst.path); os.IsNotExist(err) {
		g.logger.Info(name+" database not found, downloading", map[string]any{
			"path": inst.path,
			"url":  inst.url,
		})
		if err := g.downloadDB(inst, name); err != nil {
			return fmt.Errorf("failed to download %s database: %w", name, err)
		}
	}

	if err := g.loadDB(inst, name); err != nil {
		return fmt.Errorf("failed to load %s database: %w", name, err)
	}

	return nil
}

func (g *GeoDB) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()

	for _, inst := range []*dbInstance{g.country, g.cityIPv4, g.cityIPv6} {
		inst.mu.Lock()
		if inst.db != nil {
			_ = inst.db.Close()
		}
		inst.mu.Unlock()
	}
}

// Lookup performs a lookup. If useCity is true, tries city DB first with country fallback.
func (g *GeoDB) Lookup(ipStr string, useCity bool) (*LookupResult, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, ErrInvalidIP
	}

	if useCity {
		// Try city first, fallback to country
		if result, err := g.lookupCity(ip); err == nil {
			return result, nil
		}
		return g.lookupCountry(ip)
	}

	// Try country first, fallback to city
	if result, err := g.lookupCountry(ip); err == nil {
		return result, nil
	}
	return g.lookupCity(ip)
}

func (g *GeoDB) lookupCountry(ip netip.Addr) (*LookupResult, error) {
	g.country.mu.RLock()
	db := g.country.db
	g.country.mu.RUnlock()

	if db == nil {
		return nil, errors.New("country database not loaded")
	}

	var record CountryRecord
	if err := db.Lookup(ip).Decode(&record); err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	if record.CountryCode == "" {
		return nil, ErrIPNotFound
	}

	return &LookupResult{
		CountryCode: record.CountryCode,
	}, nil
}

func (g *GeoDB) lookupCity(ip netip.Addr) (*LookupResult, error) {
	// Select IPv4 or IPv6 database based on IP type
	var inst *dbInstance
	if ip.Is4() {
		inst = g.cityIPv4
	} else {
		inst = g.cityIPv6
	}

	inst.mu.RLock()
	db := inst.db
	inst.mu.RUnlock()

	if db == nil {
		return nil, errors.New("city database not loaded")
	}

	var record CityRecord
	if err := db.Lookup(ip).Decode(&record); err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	if record.CountryCode == "" {
		return nil, ErrIPNotFound
	}

	return &LookupResult{
		CountryCode: record.CountryCode,
		PostalCode:  record.PostCode,
	}, nil
}

func (g *GeoDB) loadDB(inst *dbInstance, name string) error {
	db, err := maxminddb.Open(inst.path)
	if err != nil {
		return err
	}

	inst.mu.Lock()
	old := inst.db
	inst.db = db
	inst.mu.Unlock()

	if old != nil {
		_ = old.Close()
	}

	g.logger.Info(name+" database loaded", map[string]any{
		"path": inst.path,
	})

	return nil
}

func (g *GeoDB) downloadDB(inst *dbInstance, name string) error {
	tmpPath := inst.path + ".tmp"

	resp, err := http.Get(inst.url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// Validate the downloaded file
	testDB, err := maxminddb.Open(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("downloaded file is invalid: %w", err)
	}
	_ = testDB.Close()

	if err := os.Rename(tmpPath, inst.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	g.logger.Info(name+" database downloaded", map[string]any{
		"path": inst.path,
		"url":  inst.url,
	})

	return nil
}

func (g *GeoDB) updateLoop(ctx context.Context) {
	defer g.wg.Done()

	ticker := time.NewTicker(g.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.logger.Info("starting scheduled database update", nil)

			dbs := []struct {
				inst *dbInstance
				name string
			}{
				{g.country, "country"},
				{g.cityIPv4, "city-ipv4"},
				{g.cityIPv6, "city-ipv6"},
			}

			for _, d := range dbs {
				if err := g.downloadDB(d.inst, d.name); err != nil {
					g.logger.Error(d.name+" database update failed", map[string]any{"error": err.Error()})
				} else if err := g.loadDB(d.inst, d.name); err != nil {
					g.logger.Error(d.name+" database reload failed", map[string]any{"error": err.Error()})
				}
			}

			g.logger.Info("database update completed", nil)
		}
	}
}
