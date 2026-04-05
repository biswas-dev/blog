package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// EngagementBeacon is what the browser sends.
type EngagementBeacon struct {
	SessionID     string            `json:"session_id"`
	Path          string            `json:"path"`
	Referrer      string            `json:"referrer"`
	StartedAt     int64             `json:"started_at"` // unix ms
	ActiveSeconds int               `json:"active_seconds"`
	TotalSeconds  int               `json:"total_seconds"`
	MaxScrollPct  int               `json:"max_scroll_pct"`
	Sections      []SectionEngaged  `json:"sections"`
	Completed     bool              `json:"completed"`
}

type SectionEngaged struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	ActiveSeconds int    `json:"active_seconds"`
	Viewed        bool   `json:"viewed"`
}

// EngagementSummary aggregates engagement across many sessions for a path.
type EngagementSummary struct {
	Path              string                    `json:"path"`
	Sessions          int                       `json:"sessions"`
	AvgActiveSeconds  float64                   `json:"avg_active_seconds"`
	AvgTotalSeconds   float64                   `json:"avg_total_seconds"`
	AvgScrollPct      float64                   `json:"avg_scroll_pct"`
	CompletionRate    float64                   `json:"completion_rate"` // % with scroll >= 90
	Sections          []SectionAggregateSummary `json:"sections"`
}

type SectionAggregateSummary struct {
	ID               string  `json:"id"`
	Text             string  `json:"text"`
	Views            int     `json:"views"`
	ViewRate         float64 `json:"view_rate"`       // % of sessions that viewed this section
	AvgActiveSeconds float64 `json:"avg_active_seconds"`
}

type EngagementService struct {
	DB *sql.DB
}

// Upsert inserts or updates an engagement record (keyed by session_id).
func (s *EngagementService) Upsert(ctx context.Context, b EngagementBeacon, ip, userAgent string, userID *int) error {
	if b.SessionID == "" || b.Path == "" || b.StartedAt == 0 {
		return nil
	}
	// Clamp scroll percentage
	if b.MaxScrollPct < 0 {
		b.MaxScrollPct = 0
	}
	if b.MaxScrollPct > 100 {
		b.MaxScrollPct = 100
	}
	// Cap time values to prevent abuse
	if b.ActiveSeconds > 86400 {
		b.ActiveSeconds = 86400
	}
	if b.TotalSeconds > 86400 {
		b.TotalSeconds = 86400
	}

	sectionsJSON, err := json.Marshal(b.Sections)
	if err != nil {
		sectionsJSON = []byte("[]")
	}

	startedAt := time.Unix(0, b.StartedAt*int64(time.Millisecond))

	_, err = s.DB.ExecContext(ctx, `
		INSERT INTO page_engagement (
			session_id, path, ip_address, user_id, referrer, user_agent,
			started_at, active_seconds, total_seconds, max_scroll_pct,
			sections, completed, updated_at
		) VALUES ($1, $2, $3::inet, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, NOW())
		ON CONFLICT (session_id, started_at) DO UPDATE SET
			active_seconds = GREATEST(page_engagement.active_seconds, EXCLUDED.active_seconds),
			total_seconds = GREATEST(page_engagement.total_seconds, EXCLUDED.total_seconds),
			max_scroll_pct = GREATEST(page_engagement.max_scroll_pct, EXCLUDED.max_scroll_pct),
			sections = EXCLUDED.sections,
			completed = page_engagement.completed OR EXCLUDED.completed,
			updated_at = NOW()
	`, b.SessionID, b.Path, ip, userID, b.Referrer, userAgent,
		startedAt, b.ActiveSeconds, b.TotalSeconds, b.MaxScrollPct,
		string(sectionsJSON), b.Completed)
	return err
}

// SummaryForPath returns aggregated engagement for a path over a window.
func (s *EngagementService) SummaryForPath(ctx context.Context, path string, since time.Time) (*EngagementSummary, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT
			COUNT(*)::int AS sessions,
			COALESCE(AVG(active_seconds), 0) AS avg_active,
			COALESCE(AVG(total_seconds), 0) AS avg_total,
			COALESCE(AVG(max_scroll_pct), 0) AS avg_scroll,
			COALESCE(AVG(CASE WHEN max_scroll_pct >= 90 THEN 1.0 ELSE 0.0 END) * 100, 0) AS completion_rate
		FROM page_engagement
		WHERE path = $1 AND started_at >= $2 AND active_seconds >= 2
	`, path, since)
	sum := &EngagementSummary{Path: path}
	if err := row.Scan(&sum.Sessions, &sum.AvgActiveSeconds, &sum.AvgTotalSeconds, &sum.AvgScrollPct, &sum.CompletionRate); err != nil {
		return nil, err
	}
	if sum.Sessions == 0 {
		return sum, nil
	}
	// Aggregate sections from JSONB
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			section->>'id' AS id,
			MAX(section->>'text') AS text,
			COUNT(*) FILTER (WHERE (section->>'viewed')::boolean)::int AS views,
			COALESCE(AVG((section->>'active_seconds')::int) FILTER (WHERE (section->>'viewed')::boolean), 0) AS avg_active
		FROM page_engagement, jsonb_array_elements(sections) AS section
		WHERE path = $1 AND started_at >= $2 AND jsonb_typeof(sections) = 'array'
		GROUP BY section->>'id'
		HAVING COUNT(*) > 0
		ORDER BY MIN((section->>'order')::int) NULLS LAST, id
	`, path, since)
	if err != nil {
		return sum, nil // return what we have
	}
	defer rows.Close()
	for rows.Next() {
		var sec SectionAggregateSummary
		var id, text sql.NullString
		if err := rows.Scan(&id, &text, &sec.Views, &sec.AvgActiveSeconds); err != nil {
			continue
		}
		sec.ID = id.String
		sec.Text = text.String
		if sum.Sessions > 0 {
			sec.ViewRate = float64(sec.Views) / float64(sum.Sessions) * 100
		}
		sum.Sections = append(sum.Sections, sec)
	}
	return sum, nil
}

// TopEngagedPaths returns paths ordered by average active time.
type PathEngagement struct {
	Path             string  `json:"path"`
	Sessions         int     `json:"sessions"`
	AvgActiveSeconds float64 `json:"avg_active_seconds"`
	AvgScrollPct     float64 `json:"avg_scroll_pct"`
	CompletionRate   float64 `json:"completion_rate"`
}

func (s *EngagementService) TopEngagedPaths(ctx context.Context, since time.Time, limit int) ([]PathEngagement, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			path,
			COUNT(*)::int AS sessions,
			AVG(active_seconds) AS avg_active,
			AVG(max_scroll_pct) AS avg_scroll,
			AVG(CASE WHEN max_scroll_pct >= 90 THEN 1.0 ELSE 0.0 END) * 100 AS completion_rate
		FROM page_engagement
		WHERE started_at >= $1 AND active_seconds >= 5
		GROUP BY path
		ORDER BY AVG(active_seconds) DESC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PathEngagement
	for rows.Next() {
		var pe PathEngagement
		if err := rows.Scan(&pe.Path, &pe.Sessions, &pe.AvgActiveSeconds, &pe.AvgScrollPct, &pe.CompletionRate); err != nil {
			continue
		}
		result = append(result, pe)
	}
	return result, nil
}
