package models

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// PageView represents a single page view event
type PageView struct {
	ViewedAt    time.Time
	IPAddress   string
	Path        string
	UserAgent   string
	Referrer    string
	UserID      *int
	ContentType string // "page", "slide", "other"
}

// AnalyticsSummary is the response for the dashboard API
type AnalyticsSummary struct {
	TimeSeries   []TimeSeriesPoint    `json:"time_series"`
	TotalViews   int64                `json:"total_views"`
	UniqueIPs    int64                `json:"unique_ips"`
	AvgDaily     float64              `json:"avg_daily"`
	TopPages     []TopItem            `json:"top_pages"`
	TopReferrers []TopItem            `json:"top_referrers"`
	ContentTypes []ContentTypeSummary `json:"content_types"`
}

// TimeSeriesPoint is a single data point in the chart
type TimeSeriesPoint struct {
	Date    string `json:"date"`
	Views   int64  `json:"views"`
	Uniques int64  `json:"uniques"`
}

// TopItem is a ranked item (page or referrer)
type TopItem struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// ContentTypeSummary breaks down views by content type
type ContentTypeSummary struct {
	ContentType string `json:"content_type"`
	Views       int64  `json:"views"`
	Uniques     int64  `json:"uniques"`
}

// LiveStats returns today's real-time counts
type LiveStats struct {
	TodayViews   int64 `json:"today_views"`
	TodayUniques int64 `json:"today_uniques"`
}

// AnalyticsService manages page view recording and querying
type AnalyticsService struct {
	db       *sql.DB
	events   chan PageView
	quit     chan struct{}
	wg       sync.WaitGroup
	stopped  bool
	mu       sync.Mutex
}

// NewAnalyticsService creates the service and starts background goroutines
func NewAnalyticsService(db *sql.DB) *AnalyticsService {
	s := &AnalyticsService{
		db:     db,
		events: make(chan PageView, 1000),
		quit:   make(chan struct{}),
	}

	// Background writer: flushes every 5s or 100 events
	s.wg.Add(1)
	go s.writerLoop()

	// Background aggregator: runs daily at 00:05
	s.wg.Add(1)
	go s.aggregatorLoop()

	// Background partition creator: ensures next month's partition exists
	s.wg.Add(1)
	go s.partitionLoop()

	return s
}

// Record enqueues a page view. Non-blocking: drops if buffer is full.
func (s *AnalyticsService) Record(pv PageView) {
	select {
	case s.events <- pv:
	default:
		// Buffer full, drop the event rather than blocking the request
	}
}

// Shutdown gracefully stops all background goroutines
func (s *AnalyticsService) Shutdown() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	close(s.quit)
	s.wg.Wait()
}

// writerLoop batches page views and inserts them
func (s *AnalyticsService) writerLoop() {
	defer s.wg.Done()

	batch := make([]PageView, 0, 100)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case pv := <-s.events:
			batch = append(batch, pv)
			if len(batch) >= 100 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		case <-s.quit:
			// Drain remaining events
			for {
				select {
				case pv := <-s.events:
					batch = append(batch, pv)
				default:
					goto done
				}
			}
		done:
			if len(batch) > 0 {
				s.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch inserts a batch of page views using a multi-row INSERT
func (s *AnalyticsService) flushBatch(batch []PageView) {
	if len(batch) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("INSERT INTO page_views (viewed_at, ip_address, path, user_agent, referrer, user_id, content_type) VALUES ")

	args := make([]interface{}, 0, len(batch)*7)
	for i, pv := range batch {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 7
		fmt.Fprintf(&b, "($%d, $%d::inet, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7)
		args = append(args, pv.ViewedAt, pv.IPAddress, pv.Path, pv.UserAgent, pv.Referrer, pv.UserID, pv.ContentType)
	}

	_, err := s.db.Exec(b.String(), args...)
	if err != nil {
		log.Printf("[analytics] batch insert failed (%d rows): %v", len(batch), err)
	}
}

// aggregatorLoop runs daily at 00:05 to aggregate yesterday's data
func (s *AnalyticsService) aggregatorLoop() {
	defer s.wg.Done()

	for {
		now := time.Now()
		// Next 00:05
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, now.Location())
		timer := time.NewTimer(time.Until(next))

		select {
		case <-timer.C:
			yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			_, err := s.db.Exec("SELECT aggregate_page_views_daily($1::date)", yesterday)
			if err != nil {
				log.Printf("[analytics] daily aggregation failed for %s: %v", yesterday, err)
			}
		case <-s.quit:
			timer.Stop()
			return
		}
	}
}

// partitionLoop ensures next month's partition exists
func (s *AnalyticsService) partitionLoop() {
	defer s.wg.Done()

	// Run immediately on startup, then monthly
	s.ensurePartitions()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.ensurePartitions()
		case <-s.quit:
			return
		}
	}
}

func (s *AnalyticsService) ensurePartitions() {
	// Ensure current month and next month partitions exist
	now := time.Now()
	nextMonth := now.AddDate(0, 1, 0)

	for _, d := range []time.Time{now, nextMonth} {
		_, err := s.db.Exec("SELECT ensure_page_views_partition($1::date)", d.Format("2006-01-02"))
		if err != nil {
			log.Printf("[analytics] partition creation failed for %s: %v", d.Format("2006-01"), err)
		}
	}
}

// GetSummary returns analytics data for the dashboard
func (s *AnalyticsService) GetSummary(period string) (*AnalyticsSummary, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	summary := &AnalyticsSummary{}

	// Time series from summary table
	rows, err := s.db.Query(`
		SELECT view_date::text, SUM(total_views), SUM(unique_visitors)
		FROM page_views_daily
		WHERE view_date >= $1::date
		GROUP BY view_date
		ORDER BY view_date`, since)
	if err != nil {
		return nil, fmt.Errorf("time series query: %w", err)
	}
	defer rows.Close()

	var totalViews, totalUniques int64
	for rows.Next() {
		var pt TimeSeriesPoint
		if err := rows.Scan(&pt.Date, &pt.Views, &pt.Uniques); err != nil {
			return nil, fmt.Errorf("time series scan: %w", err)
		}
		totalViews += pt.Views
		totalUniques += pt.Uniques
		summary.TimeSeries = append(summary.TimeSeries, pt)
	}
	summary.TotalViews = totalViews
	summary.UniqueIPs = totalUniques
	if days > 0 {
		summary.AvgDaily = float64(totalViews) / float64(days)
	}

	// Top pages
	topPages, err := s.db.Query(`
		SELECT path, SUM(total_views) AS views
		FROM page_views_daily
		WHERE view_date >= $1::date
		GROUP BY path
		ORDER BY views DESC
		LIMIT 10`, since)
	if err != nil {
		return nil, fmt.Errorf("top pages query: %w", err)
	}
	defer topPages.Close()

	for topPages.Next() {
		var item TopItem
		if err := topPages.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top pages scan: %w", err)
		}
		summary.TopPages = append(summary.TopPages, item)
	}

	// Top referrers from raw table (not in summary)
	topRefs, err := s.db.Query(`
		SELECT COALESCE(NULLIF(referrer, ''), '(direct)') AS ref, COUNT(*) AS cnt
		FROM page_views
		WHERE viewed_at >= $1::date
		GROUP BY ref
		ORDER BY cnt DESC
		LIMIT 10`, since)
	if err != nil {
		return nil, fmt.Errorf("top referrers query: %w", err)
	}
	defer topRefs.Close()

	for topRefs.Next() {
		var item TopItem
		if err := topRefs.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top referrers scan: %w", err)
		}
		summary.TopReferrers = append(summary.TopReferrers, item)
	}

	// Content type breakdown
	ctRows, err := s.db.Query(`
		SELECT content_type, SUM(total_views), SUM(unique_visitors)
		FROM page_views_daily
		WHERE view_date >= $1::date
		GROUP BY content_type
		ORDER BY SUM(total_views) DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("content type query: %w", err)
	}
	defer ctRows.Close()

	for ctRows.Next() {
		var ct ContentTypeSummary
		if err := ctRows.Scan(&ct.ContentType, &ct.Views, &ct.Uniques); err != nil {
			return nil, fmt.Errorf("content type scan: %w", err)
		}
		summary.ContentTypes = append(summary.ContentTypes, ct)
	}

	return summary, nil
}

// GetTodayLiveCount returns today's views and unique visitors from the raw table
func (s *AnalyticsService) GetTodayLiveCount() (*LiveStats, error) {
	today := time.Now().Format("2006-01-02")
	stats := &LiveStats{}

	err := s.db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT ip_address)
		FROM page_views
		WHERE viewed_at >= $1::date`, today).Scan(&stats.TodayViews, &stats.TodayUniques)
	if err != nil {
		return nil, fmt.Errorf("today live count: %w", err)
	}

	return stats, nil
}

// ContentTypeForPath derives content_type from a URL path
func ContentTypeForPath(path string) string {
	if strings.HasPrefix(path, "/blog/") || path == "/blog" {
		return "page"
	}
	if strings.HasPrefix(path, "/slides/") || path == "/slides" {
		return "slide"
	}
	return "other"
}

func parsePeriodDays(period string) int {
	switch period {
	case "7d":
		return 7
	case "30d":
		return 30
	case "90d":
		return 90
	case "1y":
		return 365
	default:
		return 30
	}
}
