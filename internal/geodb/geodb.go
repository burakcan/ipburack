package geodb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

var (
	ErrInvalidIP  = errors.New("invalid IP address")
	ErrIPNotFound = errors.New("IP not found in database")
)

// Record matches the structure in geolite2-geo-whois-asn-country MMDB
type Record struct {
	CountryCode string `maxminddb:"country_code"`
}

type LookupResult struct {
	IP          string `json:"ip"`
	CountryCode string `json:"country_code"`
}

type Logger interface {
	Info(message string, data map[string]any)
	Error(message string, data map[string]any)
}

type GeoDB struct {
	db             *maxminddb.Reader
	mu             sync.RWMutex
	path           string
	url            string
	updateInterval time.Duration
	logger         Logger
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func New(path, url string, updateInterval time.Duration, logger Logger) *GeoDB {
	return &GeoDB{
		path:           path,
		url:            url,
		updateInterval: updateInterval,
		logger:         logger,
	}
}

func (g *GeoDB) Start(ctx context.Context) error {
	dir := filepath.Dir(g.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if _, err := os.Stat(g.path); os.IsNotExist(err) {
		g.logger.Info("database not found, downloading", map[string]any{
			"path": g.path,
			"url":  g.url,
		})
		if err := g.download(); err != nil {
			return fmt.Errorf("failed to download database: %w", err)
		}
	}

	if err := g.load(); err != nil {
		return fmt.Errorf("failed to load database: %w", err)
	}

	updateCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.wg.Add(1)
	go g.updateLoop(updateCtx)

	return nil
}

func (g *GeoDB) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()

	g.mu.Lock()
	if g.db != nil {
		g.db.Close()
	}
	g.mu.Unlock()
}

func (g *GeoDB) Lookup(ipStr string) (*LookupResult, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, ErrInvalidIP
	}

	g.mu.RLock()
	db := g.db
	g.mu.RUnlock()

	if db == nil {
		return nil, errors.New("database not loaded")
	}

	var record Record
	err := db.Lookup(ip, &record)
	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	if record.CountryCode == "" {
		return nil, ErrIPNotFound
	}

	return &LookupResult{
		IP:          ipStr,
		CountryCode: record.CountryCode,
	}, nil
}

func (g *GeoDB) load() error {
	db, err := maxminddb.Open(g.path)
	if err != nil {
		return err
	}

	g.mu.Lock()
	old := g.db
	g.db = db
	g.mu.Unlock()

	if old != nil {
		old.Close()
	}

	g.logger.Info("database loaded", map[string]any{
		"path": g.path,
	})

	return nil
}

func (g *GeoDB) download() error {
	tmpPath := g.path + ".tmp"

	resp, err := http.Get(g.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Validate the downloaded file
	testDB, err := maxminddb.Open(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded file is invalid: %w", err)
	}
	testDB.Close()

	if err := os.Rename(tmpPath, g.path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	g.logger.Info("database downloaded", map[string]any{
		"path": g.path,
		"url":  g.url,
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

			if err := g.download(); err != nil {
				g.logger.Error("database update failed", map[string]any{
					"error": err.Error(),
				})
				continue
			}

			if err := g.load(); err != nil {
				g.logger.Error("failed to reload database after update", map[string]any{
					"error": err.Error(),
				})
				continue
			}

			g.logger.Info("database update completed", nil)
		}
	}
}
