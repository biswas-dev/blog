package models

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// IPRule represents a ban or allow rule for an IP address.
type IPRule struct {
	ID        int    `json:"id"`
	IPAddress string `json:"ip_address"`
	Action    string `json:"action"` // "ban" | "allow"
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// IPBanCache holds in-memory sets of banned and allowed IPs.
// It lives in models so middleware can import it without circular deps.
// sync.RWMutex + plain map is used (read-heavy workload).
type IPBanCache struct {
	mu      sync.RWMutex
	banned  map[string]struct{}
	allowed map[string]struct{}
}

// NewIPBanCache creates an empty cache.
func NewIPBanCache() *IPBanCache {
	return &IPBanCache{
		banned:  make(map[string]struct{}),
		allowed: make(map[string]struct{}),
	}
}

// IsBanned reports whether ip is in the ban set.
func (c *IPBanCache) IsBanned(ip string) bool {
	c.mu.RLock()
	_, ok := c.banned[ip]
	c.mu.RUnlock()
	return ok
}

// IsAllowed reports whether ip is in the allow set.
func (c *IPBanCache) IsAllowed(ip string) bool {
	c.mu.RLock()
	_, ok := c.allowed[ip]
	c.mu.RUnlock()
	return ok
}

// SetBanned adds ip to the ban set and removes it from the allow set.
func (c *IPBanCache) SetBanned(ip string) {
	c.mu.Lock()
	c.banned[ip] = struct{}{}
	delete(c.allowed, ip)
	c.mu.Unlock()
}

// SetAllowed adds ip to the allow set and removes it from the ban set.
func (c *IPBanCache) SetAllowed(ip string) {
	c.mu.Lock()
	c.allowed[ip] = struct{}{}
	delete(c.banned, ip)
	c.mu.Unlock()
}

// Remove removes ip from both sets.
func (c *IPBanCache) Remove(ip string) {
	c.mu.Lock()
	delete(c.banned, ip)
	delete(c.allowed, ip)
	c.mu.Unlock()
}

// IPRulesService manages IP ban/allow rules with DB persistence and in-memory caching.
type IPRulesService struct {
	DB    *sql.DB
	Cache *IPBanCache
	quit  chan struct{}
	wg    sync.WaitGroup
}

// NewIPRulesService creates the service and starts the auto-ban goroutine.
func NewIPRulesService(db *sql.DB, cache *IPBanCache) *IPRulesService {
	s := &IPRulesService{
		DB:    db,
		Cache: cache,
		quit:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.autoBanLoop()
	return s
}

// Shutdown stops the auto-ban goroutine and waits for it to finish.
func (s *IPRulesService) Shutdown() {
	close(s.quit)
	s.wg.Wait()
}

// LoadForCache loads all ip_rules into the in-memory cache. Call once at startup before
// registering middleware.
func (s *IPRulesService) LoadForCache() error {
	rows, err := s.DB.Query(`SELECT host(ip_address), action FROM ip_rules`)
	if err != nil {
		return fmt.Errorf("ip_rules load: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ip, action string
		if err := rows.Scan(&ip, &action); err != nil {
			return fmt.Errorf("ip_rules scan: %w", err)
		}
		switch action {
		case "ban":
			s.Cache.SetBanned(ip)
		case "allow":
			s.Cache.SetAllowed(ip)
		}
	}
	return rows.Err()
}

// BanIP upserts a ban rule and updates the cache.
func (s *IPRulesService) BanIP(ip, reason string) error {
	_, err := s.DB.Exec(`
		INSERT INTO ip_rules (ip_address, action, reason)
		VALUES ($1::inet, 'ban', $2)
		ON CONFLICT (ip_address) DO UPDATE
			SET action = 'ban', reason = EXCLUDED.reason, updated_at = NOW()`,
		ip, reason)
	if err != nil {
		return fmt.Errorf("ban ip %s: %w", ip, err)
	}
	s.Cache.SetBanned(ip)
	return nil
}

// AllowIP upserts an allow rule and updates the cache.
func (s *IPRulesService) AllowIP(ip, reason string) error {
	_, err := s.DB.Exec(`
		INSERT INTO ip_rules (ip_address, action, reason)
		VALUES ($1::inet, 'allow', $2)
		ON CONFLICT (ip_address) DO UPDATE
			SET action = 'allow', reason = EXCLUDED.reason, updated_at = NOW()`,
		ip, reason)
	if err != nil {
		return fmt.Errorf("allow ip %s: %w", ip, err)
	}
	s.Cache.SetAllowed(ip)
	return nil
}

// RemoveRule deletes the rule for ip and updates the cache.
func (s *IPRulesService) RemoveRule(ip string) error {
	_, err := s.DB.Exec(`DELETE FROM ip_rules WHERE ip_address = $1::inet`, ip)
	if err != nil {
		return fmt.Errorf("remove rule %s: %w", ip, err)
	}
	s.Cache.Remove(ip)
	return nil
}

// ListRules returns all rules ordered by created_at DESC.
func (s *IPRulesService) ListRules() ([]IPRule, error) {
	rows, err := s.DB.Query(`
		SELECT id, host(ip_address), action, reason,
			created_at::text, updated_at::text
		FROM ip_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	var rules []IPRule
	for rows.Next() {
		var r IPRule
		if err := rows.Scan(&r.ID, &r.IPAddress, &r.Action, &r.Reason, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list rules scan: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// autoBanLoop runs runAutoBan at startup and then every 15 minutes.
func (s *IPRulesService) autoBanLoop() {
	defer s.wg.Done()

	threshold := 100
	if v := os.Getenv("IP_BAN_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threshold = n
		}
	}

	s.runAutoBan(threshold)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runAutoBan(threshold)
		case <-s.quit:
			return
		}
	}
}

// runAutoBan queries for IPs with >= threshold suspicious requests in the last 24h
// that are not already in ip_rules, and bans them.
func (s *IPRulesService) runAutoBan(threshold int) {
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT host(ip_address) AS ip, COUNT(*) AS cnt
		FROM page_views
		WHERE viewed_at >= NOW() - INTERVAL '24 hours'
		  AND %s
		  AND ip_address NOT IN (SELECT ip_address FROM ip_rules)
		GROUP BY ip_address
		HAVING COUNT(*) >= $1`, suspiciousPathSQL), threshold)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		var cnt int64
		if err := rows.Scan(&ip, &cnt); err != nil {
			continue
		}
		reason := fmt.Sprintf("auto-ban: %d suspicious requests in 24h", cnt)
		_ = s.BanIP(ip, reason)
	}
}
