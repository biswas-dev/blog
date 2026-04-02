package models

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CrawlerRule represents a rule for controlling crawler access.
type CrawlerRule struct {
	ID             int    `json:"id"`
	CrawlerPattern string `json:"crawler_pattern"`
	Action         string `json:"action"` // "allow", "block", "time_restrict"
	TimeStart      *int   `json:"time_start"`
	TimeEnd        *int   `json:"time_end"`
	Reason         string `json:"reason"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// CrawlerRuleService manages crawler access rules with DB persistence and in-memory caching.
type CrawlerRuleService struct {
	DB    *sql.DB
	mu    sync.RWMutex
	rules []CrawlerRule
}

// NewCrawlerRuleService creates the service and loads rules from DB into memory.
func NewCrawlerRuleService(db *sql.DB) (*CrawlerRuleService, error) {
	s := &CrawlerRuleService{DB: db}
	if err := s.reload(); err != nil {
		return nil, fmt.Errorf("crawler rules init: %w", err)
	}
	return s, nil
}

// reload reads all rules from the database into the in-memory cache.
func (s *CrawlerRuleService) reload() error {
	rows, err := s.DB.Query(`
		SELECT id, crawler_pattern, action,
			time_start, time_end, reason,
			created_at::text, updated_at::text
		FROM crawler_rules
		ORDER BY created_at DESC`)
	if err != nil {
		return fmt.Errorf("list crawler rules: %w", err)
	}
	defer rows.Close()

	var rules []CrawlerRule
	for rows.Next() {
		var r CrawlerRule
		if err := rows.Scan(&r.ID, &r.CrawlerPattern, &r.Action,
			&r.TimeStart, &r.TimeEnd, &r.Reason,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return fmt.Errorf("scan crawler rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("crawler rules rows: %w", err)
	}

	s.mu.Lock()
	s.rules = rules
	s.mu.Unlock()
	return nil
}

// GetAll returns all crawler rules.
func (s *CrawlerRuleService) GetAll() ([]CrawlerRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CrawlerRule, len(s.rules))
	copy(out, s.rules)
	return out, nil
}

// Create adds a new crawler rule.
func (s *CrawlerRuleService) Create(pattern, action string, timeStart, timeEnd *int, reason string) error {
	_, err := s.DB.Exec(`
		INSERT INTO crawler_rules (crawler_pattern, action, time_start, time_end, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		pattern, action, timeStart, timeEnd, reason)
	if err != nil {
		return fmt.Errorf("create crawler rule: %w", err)
	}
	return s.reload()
}

// Update modifies an existing crawler rule.
func (s *CrawlerRuleService) Update(id int, action string, timeStart, timeEnd *int, reason string) error {
	_, err := s.DB.Exec(`
		UPDATE crawler_rules
		SET action = $1, time_start = $2, time_end = $3, reason = $4, updated_at = NOW()
		WHERE id = $5`,
		action, timeStart, timeEnd, reason, id)
	if err != nil {
		return fmt.Errorf("update crawler rule: %w", err)
	}
	return s.reload()
}

// Delete removes a crawler rule by ID.
func (s *CrawlerRuleService) Delete(id int) error {
	_, err := s.DB.Exec(`DELETE FROM crawler_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete crawler rule: %w", err)
	}
	return s.reload()
}

// IsBlocked checks if a crawler type should be blocked right now.
// It checks rules whose pattern matches the crawlerType (case-insensitive substring).
// Returns true if any matching rule blocks the crawler at the current UTC hour.
func (s *CrawlerRuleService) IsBlocked(crawlerType string) bool {
	if crawlerType == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	crawlerLower := strings.ToLower(crawlerType)
	nowHour := time.Now().UTC().Hour()

	for _, r := range s.rules {
		if !strings.Contains(crawlerLower, strings.ToLower(r.CrawlerPattern)) {
			continue
		}
		switch r.Action {
		case "block":
			return true
		case "time_restrict":
			if r.TimeStart != nil && r.TimeEnd != nil {
				start, end := *r.TimeStart, *r.TimeEnd
				if start <= end {
					// e.g. 2-14: block during 2..14
					if nowHour >= start && nowHour < end {
						return true
					}
				} else {
					// wraps midnight, e.g. 22-6: block during 22..24 and 0..6
					if nowHour >= start || nowHour < end {
						return true
					}
				}
			}
		}
	}
	return false
}
