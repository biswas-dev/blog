package models

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// SiteSettingsService provides cached read/write access to the site_settings
// key-value table. Values are cached in memory and refreshed on write or
// after the TTL expires.
type SiteSettingsService struct {
	DB       *sql.DB
	mu       sync.RWMutex
	cache    map[string]cachedVal
	cacheTTL time.Duration
}

type cachedVal struct {
	value   string
	fetched time.Time
}

const defaultCacheTTL = 5 * time.Minute

// NewSiteSettingsService creates a SiteSettingsService with a default 5-minute
// cache TTL.
func NewSiteSettingsService(db *sql.DB) *SiteSettingsService {
	return &SiteSettingsService{
		DB:       db,
		cache:    make(map[string]cachedVal),
		cacheTTL: defaultCacheTTL,
	}
}

// Get returns the value for a key, or fallback if unset / not found.
func (s *SiteSettingsService) Get(key, fallback string) string {
	s.mu.RLock()
	if c, ok := s.cache[key]; ok && time.Since(c.fetched) < s.cacheTTL {
		s.mu.RUnlock()
		return c.value
	}
	s.mu.RUnlock()

	var value string
	err := s.DB.QueryRow("SELECT value FROM site_settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		return fallback
	}
	s.mu.Lock()
	s.cache[key] = cachedVal{value: value, fetched: time.Now()}
	s.mu.Unlock()
	return value
}

// Set writes a key-value pair (upsert) and updates the cache.
func (s *SiteSettingsService) Set(key, value string) error {
	_, err := s.DB.Exec(`
		INSERT INTO site_settings (key, value, updated_at) VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP`,
		key, value)
	if err != nil {
		return fmt.Errorf("site_settings set %q: %w", key, err)
	}
	s.mu.Lock()
	s.cache[key] = cachedVal{value: value, fetched: time.Now()}
	s.mu.Unlock()
	return nil
}

// GetInt returns a numeric setting, clamped between min and max.
func (s *SiteSettingsService) GetInt(key string, fallback, min, max int) int {
	raw := s.Get(key, strconv.Itoa(fallback))
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
