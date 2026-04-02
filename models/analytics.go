package models

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const dateFormat = "2006-01-02"

// suspiciousPathSQL is a reusable SQL fragment that matches exploit probe paths
const suspiciousPathSQL = `(
	path LIKE '%.php' OR path LIKE '%.asp' OR path LIKE '%.aspx' OR path LIKE '%.jsp'
	OR path LIKE '%/.git%' OR path LIKE '%.env%' OR path LIKE '%/wp-%'
	OR path LIKE '%/xmlrpc%' OR path LIKE '%/phpmyadmin%'
	OR path LIKE '%passwd%' OR path LIKE '%/etc/%'
	OR path LIKE '%.bak' OR path LIKE '%.sql' OR path LIKE '%.conf'
)`

// PageView represents a single page view event
type PageView struct {
	ViewedAt    time.Time
	IPAddress   string
	Path        string
	UserAgent   string
	Referrer    string
	UserID      *int
	ContentType string // "page", "slide", "other"
	CrawlerType string // e.g. "GoogleBot", "GPTBot", "" for humans
}

// AnalyticsSummary is the response for the dashboard API
type AnalyticsSummary struct {
	TimeSeries     []TimeSeriesPoint    `json:"time_series"`
	TotalViews     int64                `json:"total_views"`
	UniqueIPs      int64                `json:"unique_ips"`
	AvgDaily       float64              `json:"avg_daily"`
	TopPages       []TopItem            `json:"top_pages"`
	TopReferrers   []TopItem            `json:"top_referrers"`
	ContentTypes   []ContentTypeSummary `json:"content_types"`
	TopVisitors    []TopVisitor         `json:"top_visitors"`
	HourlyActivity     []HourlyActivity     `json:"hourly_activity"`
	Browsers           []BrowserStat        `json:"browsers"`
	TopBlogPosts       []TopItem            `json:"top_blog_posts"`
	TopSlides          []TopItem            `json:"top_slides"`
	SuspiciousRequests []TopItem            `json:"suspicious_requests"`
	SuspiciousVisitors []TopVisitor         `json:"suspicious_visitors"`
	SuspiciousCount    int64                `json:"suspicious_count"`
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

// TopVisitor represents a top visitor IP with activity summary
type TopVisitor struct {
	IPAddress  string `json:"ip_address"`
	Views      int64  `json:"views"`
	LastSeen   string `json:"last_seen"`
	TopPath    string `json:"top_path"`
	UserAgent  string `json:"user_agent"`
	RuleAction string `json:"rule_action"` // "", "ban", or "allow"
}

// VisitorDetail represents a single page view by a specific IP
type VisitorDetail struct {
	Path      string `json:"path"`
	ViewedAt  string `json:"viewed_at"`
	UserAgent string `json:"user_agent"`
	Referrer  string `json:"referrer"`
}

// HourlyActivity represents views aggregated by hour of day
type HourlyActivity struct {
	Hour  int   `json:"hour"`
	Views int64 `json:"views"`
}

// BrowserStat represents browser usage breakdown
type BrowserStat struct {
	Name    string  `json:"name"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

// CrawlerStat represents aggregated stats for a single crawler type
type CrawlerStat struct {
	CrawlerType string `json:"crawler_type"`
	Requests    int64  `json:"requests"`
	UniquePaths int64  `json:"unique_paths"`
	LastSeen    string `json:"last_seen"`
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

	// One-time backfill of page_views_daily for any historical dates missing aggregations
	go s.BackfillDailyAggregates()

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
	b.WriteString("INSERT INTO page_views (viewed_at, ip_address, path, user_agent, referrer, user_id, content_type, crawler_type) VALUES ")

	args := make([]interface{}, 0, len(batch)*8)
	for i, pv := range batch {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 8
		fmt.Fprintf(&b, "($%d, $%d::inet, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8)
		args = append(args, pv.ViewedAt, pv.IPAddress, pv.Path, pv.UserAgent, pv.Referrer, pv.UserID, pv.ContentType, pv.CrawlerType)
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
			yesterday := time.Now().AddDate(0, 0, -1).Format(dateFormat)
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
		_, err := s.db.Exec("SELECT ensure_page_views_partition($1::date)", d.Format(dateFormat))
		if err != nil {
			log.Printf("[analytics] partition creation failed for %s: %v", d.Format("2006-01"), err)
		}
	}
}

// getTimeSeries fetches the time-series data for the given since date.
// Returns (timeSeries, totalViews, totalUniques, error).
func (s *AnalyticsService) getTimeSeries(since string) ([]TimeSeriesPoint, int64, int64, error) {
	rows, err := s.db.Query(`
		WITH daily AS (
			SELECT view_date::text AS date, SUM(total_views) AS views, SUM(unique_visitors) AS uniques
			FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE
			GROUP BY view_date
		),
		today AS (
			SELECT CURRENT_DATE::text AS date, COUNT(*) AS views, COUNT(DISTINCT ip_address) AS uniques
			FROM page_views WHERE viewed_at >= CURRENT_DATE
			AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'ban')
		)
		SELECT date, views, uniques FROM daily
		UNION ALL
		SELECT date, views, uniques FROM today WHERE views > 0
		ORDER BY date`, since)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("time series query: %w", err)
	}
	defer rows.Close()

	var series []TimeSeriesPoint
	var totalViews, totalUniques int64
	for rows.Next() {
		var pt TimeSeriesPoint
		if err := rows.Scan(&pt.Date, &pt.Views, &pt.Uniques); err != nil {
			return nil, 0, 0, fmt.Errorf("time series scan: %w", err)
		}
		totalViews += pt.Views
		totalUniques += pt.Uniques
		series = append(series, pt)
	}
	return series, totalViews, totalUniques, nil
}

// getTopPages fetches the top-10 pages for the given since date, excluding exploit probe paths.
func (s *AnalyticsService) getTopPages(since string) ([]TopItem, error) {
	rows, err := s.db.Query(fmt.Sprintf(`
		WITH past AS (
			SELECT path, SUM(total_views) AS views FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE AND NOT %s GROUP BY path
		),
		today AS (
			SELECT path, COUNT(*) AS views FROM page_views WHERE viewed_at >= CURRENT_DATE AND NOT %s GROUP BY path
		),
		combined AS (
			SELECT path, SUM(views) AS views FROM (SELECT * FROM past UNION ALL SELECT * FROM today) t GROUP BY path
		)
		SELECT path, views FROM combined ORDER BY views DESC LIMIT 10`,
		suspiciousPathSQL, suspiciousPathSQL), since)
	if err != nil {
		return nil, fmt.Errorf("top pages query: %w", err)
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top pages scan: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// getTopReferrers fetches the top-10 referrers for the given since date.
func (s *AnalyticsService) getTopReferrers(since string) ([]TopItem, error) {
	rows, err := s.db.Query(`
		SELECT COALESCE(NULLIF(referrer, ''), '(direct)') AS ref, COUNT(*) AS cnt
		FROM page_views
		WHERE viewed_at >= $1::date
		GROUP BY ref
		ORDER BY cnt DESC
		LIMIT 10`, since)
	if err != nil {
		return nil, fmt.Errorf("top referrers query: %w", err)
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top referrers scan: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// getContentTypes fetches content-type breakdown for the given since date.
func (s *AnalyticsService) getContentTypes(since string) ([]ContentTypeSummary, error) {
	rows, err := s.db.Query(`
		WITH past AS (
			SELECT content_type, SUM(total_views) AS views, SUM(unique_visitors) AS uniques
			FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE
			GROUP BY content_type
		),
		today AS (
			SELECT content_type, COUNT(*) AS views, COUNT(DISTINCT ip_address) AS uniques
			FROM page_views WHERE viewed_at >= CURRENT_DATE GROUP BY content_type
		),
		combined AS (
			SELECT content_type, SUM(views) AS views, SUM(uniques) AS uniques
			FROM (SELECT * FROM past UNION ALL SELECT * FROM today) t GROUP BY content_type
		)
		SELECT content_type, views, uniques FROM combined ORDER BY views DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("content type query: %w", err)
	}
	defer rows.Close()

	var items []ContentTypeSummary
	for rows.Next() {
		var ct ContentTypeSummary
		if err := rows.Scan(&ct.ContentType, &ct.Views, &ct.Uniques); err != nil {
			return nil, fmt.Errorf("content type scan: %w", err)
		}
		items = append(items, ct)
	}
	return items, nil
}

// getTopBlogPosts fetches the top-15 blog post paths for the given since date.
func (s *AnalyticsService) getTopBlogPosts(since string) ([]TopItem, error) {
	rows, err := s.db.Query(`
		WITH past AS (
			SELECT path, SUM(total_views) AS views FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE AND path LIKE '/blog/%' GROUP BY path
		),
		today AS (
			SELECT path, COUNT(*) AS views FROM page_views WHERE viewed_at >= CURRENT_DATE AND path LIKE '/blog/%' GROUP BY path
		),
		combined AS (
			SELECT path, SUM(views) AS views FROM (SELECT * FROM past UNION ALL SELECT * FROM today) t GROUP BY path
		)
		SELECT path, views FROM combined ORDER BY views DESC LIMIT 15`, since)
	if err != nil {
		return nil, fmt.Errorf("top blog posts query: %w", err)
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top blog posts scan: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// getTopSlides fetches the top-10 slide paths for the given since date.
func (s *AnalyticsService) getTopSlides(since string) ([]TopItem, error) {
	rows, err := s.db.Query(`
		WITH past AS (
			SELECT path, SUM(total_views) AS views FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE AND path LIKE '/slides/%' GROUP BY path
		),
		today AS (
			SELECT path, COUNT(*) AS views FROM page_views WHERE viewed_at >= CURRENT_DATE AND path LIKE '/slides/%' GROUP BY path
		),
		combined AS (
			SELECT path, SUM(views) AS views FROM (SELECT * FROM past UNION ALL SELECT * FROM today) t GROUP BY path
		)
		SELECT path, views FROM combined ORDER BY views DESC LIMIT 10`, since)
	if err != nil {
		return nil, fmt.Errorf("top slides query: %w", err)
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("top slides scan: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// getSuspiciousRequests fetches the top-20 exploit probe paths for the given since date.
func (s *AnalyticsService) getSuspiciousRequests(since string) ([]TopItem, error) {
	rows, err := s.db.Query(fmt.Sprintf(`
		WITH past AS (
			SELECT path, SUM(total_views) AS views FROM page_views_daily
			WHERE view_date >= $1::date AND view_date < CURRENT_DATE AND %s GROUP BY path
		),
		today AS (
			SELECT path, COUNT(*) AS views FROM page_views WHERE viewed_at >= CURRENT_DATE AND %s
			AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'allow')
			GROUP BY path
		),
		combined AS (
			SELECT path, SUM(views) AS views FROM (SELECT * FROM past UNION ALL SELECT * FROM today) t GROUP BY path
		)
		SELECT path, views FROM combined ORDER BY views DESC LIMIT 20`,
		suspiciousPathSQL, suspiciousPathSQL), since)
	if err != nil {
		return nil, fmt.Errorf("suspicious requests query: %w", err)
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("suspicious requests scan: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// getSuspiciousVisitors fetches IPs whose most-visited path is an exploit probe path.
func (s *AnalyticsService) getSuspiciousVisitors(since string) ([]TopVisitor, error) {
	rows, err := s.db.Query(`
		WITH visitor_summary AS (
			SELECT ip_address, ip_address::text AS ip_text, COUNT(*) AS views, MAX(viewed_at)::text AS last_seen,
				MODE() WITHIN GROUP (ORDER BY path) AS top_path,
				(ARRAY_AGG(user_agent ORDER BY viewed_at DESC))[1] AS user_agent
			FROM page_views WHERE viewed_at >= $1::date
			AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'allow')
			GROUP BY ip_address
		)
		SELECT vs.ip_text, vs.views, vs.last_seen, vs.top_path, vs.user_agent,
			COALESCE(ir.action, '') AS rule_action
		FROM visitor_summary vs
		LEFT JOIN ip_rules ir ON ir.ip_address = vs.ip_address
		WHERE (
			vs.top_path LIKE '%.php' OR vs.top_path LIKE '%.asp' OR vs.top_path LIKE '%.aspx' OR vs.top_path LIKE '%.jsp'
			OR vs.top_path LIKE '%/.git%' OR vs.top_path LIKE '%.env%' OR vs.top_path LIKE '%/wp-%'
			OR vs.top_path LIKE '%/xmlrpc%' OR vs.top_path LIKE '%/phpmyadmin%'
			OR vs.top_path LIKE '%passwd%' OR vs.top_path LIKE '%/etc/%'
			OR vs.top_path LIKE '%.bak' OR vs.top_path LIKE '%.sql' OR vs.top_path LIKE '%.conf'
		)
		ORDER BY vs.views DESC LIMIT 500`, since)
	if err != nil {
		return nil, fmt.Errorf("suspicious visitors query: %w", err)
	}
	defer rows.Close()

	var visitors []TopVisitor
	for rows.Next() {
		var v TopVisitor
		if err := rows.Scan(&v.IPAddress, &v.Views, &v.LastSeen, &v.TopPath, &v.UserAgent, &v.RuleAction); err != nil {
			return nil, fmt.Errorf("suspicious visitors scan: %w", err)
		}
		visitors = append(visitors, v)
	}
	return visitors, nil
}

// getSuspiciousCount returns the total number of exploit probe requests in the period.
func (s *AnalyticsService) getSuspiciousCount(since string) (int64, error) {
	var count int64
	err := s.db.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*) FROM page_views
		WHERE viewed_at >= $1::date AND %s
		AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'ban')
		AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'allow')`,
		suspiciousPathSQL), since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("suspicious count query: %w", err)
	}
	return count, nil
}

// GetSummary returns analytics data for the dashboard
func (s *AnalyticsService) GetSummary(period string) (*AnalyticsSummary, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	summary := &AnalyticsSummary{}

	timeSeries, totalViews, totalUniques, err := s.getTimeSeries(since)
	if err != nil {
		return nil, err
	}
	summary.TimeSeries = timeSeries
	summary.TotalViews = totalViews
	summary.UniqueIPs = totalUniques
	if days > 0 {
		summary.AvgDaily = float64(totalViews) / float64(days)
	}

	if summary.TopPages, err = s.getTopPages(since); err != nil {
		return nil, err
	}

	if summary.TopReferrers, err = s.getTopReferrers(since); err != nil {
		return nil, err
	}

	if summary.ContentTypes, err = s.getContentTypes(since); err != nil {
		return nil, err
	}

	// Top visitors (non-fatal on error)
	if visitors, err := s.GetTopVisitors(period); err != nil {
		log.Printf("[analytics] top visitors query failed: %v", err)
	} else {
		summary.TopVisitors = visitors
	}

	// Hourly activity (non-fatal on error)
	if hourly, err := s.GetHourlyActivity(period); err != nil {
		log.Printf("[analytics] hourly activity query failed: %v", err)
	} else {
		summary.HourlyActivity = hourly
	}

	// Browser stats (non-fatal on error)
	if browsers, err := s.GetBrowserStats(period); err != nil {
		log.Printf("[analytics] browser stats query failed: %v", err)
	} else {
		summary.Browsers = browsers
	}

	// Top blog posts (non-fatal on error)
	if blogPosts, err := s.getTopBlogPosts(since); err != nil {
		log.Printf("[analytics] top blog posts query failed: %v", err)
	} else {
		summary.TopBlogPosts = blogPosts
	}

	// Top slides (non-fatal on error)
	if slides, err := s.getTopSlides(since); err != nil {
		log.Printf("[analytics] top slides query failed: %v", err)
	} else {
		summary.TopSlides = slides
	}

	// Suspicious requests (non-fatal on error)
	if suspReqs, err := s.getSuspiciousRequests(since); err != nil {
		log.Printf("[analytics] suspicious requests query failed: %v", err)
	} else {
		summary.SuspiciousRequests = suspReqs
	}

	// Suspicious visitors (non-fatal on error)
	if suspVisitors, err := s.getSuspiciousVisitors(since); err != nil {
		log.Printf("[analytics] suspicious visitors query failed: %v", err)
	} else {
		summary.SuspiciousVisitors = suspVisitors
	}

	// Suspicious count (non-fatal on error)
	if count, err := s.getSuspiciousCount(since); err != nil {
		log.Printf("[analytics] suspicious count query failed: %v", err)
	} else {
		summary.SuspiciousCount = count
	}

	return summary, nil
}

// GetTodayLiveCount returns today's views and unique visitors from the raw table
func (s *AnalyticsService) GetTodayLiveCount() (*LiveStats, error) {
	today := time.Now().Format(dateFormat)
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

// BackfillDailyAggregates populates page_views_daily for historical dates
// that have raw data but no aggregated rows. Safe to run multiple times.
func (s *AnalyticsService) BackfillDailyAggregates() {
	_, err := s.db.Exec(`
		INSERT INTO page_views_daily (view_date, path, content_type, total_views, unique_visitors)
		SELECT viewed_at::date, path, content_type, COUNT(*), COUNT(DISTINCT ip_address)
		FROM page_views
		WHERE viewed_at < CURRENT_DATE
		GROUP BY viewed_at::date, path, content_type
		ON CONFLICT (view_date, path, content_type) DO NOTHING`)
	if err != nil {
		log.Printf("[analytics] backfill daily aggregates failed: %v", err)
	} else {
		log.Printf("[analytics] backfill daily aggregates completed")
	}
}

// GetTopVisitors returns the top 20 IPs by view count for the given period
func (s *AnalyticsService) GetTopVisitors(period string) ([]TopVisitor, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	rows, err := s.db.Query(`
		SELECT ip_address::text, COUNT(*) AS views,
			MAX(viewed_at)::text AS last_seen,
			MODE() WITHIN GROUP (ORDER BY path) AS top_path,
			(ARRAY_AGG(user_agent ORDER BY viewed_at DESC))[1] AS user_agent
		FROM page_views WHERE viewed_at >= $1::date
		AND ip_address NOT IN (SELECT ip_address FROM ip_rules WHERE action = 'ban')
		GROUP BY ip_address ORDER BY views DESC LIMIT 20`, since)
	if err != nil {
		return nil, fmt.Errorf("top visitors query: %w", err)
	}
	defer rows.Close()

	var visitors []TopVisitor
	for rows.Next() {
		var v TopVisitor
		if err := rows.Scan(&v.IPAddress, &v.Views, &v.LastSeen, &v.TopPath, &v.UserAgent); err != nil {
			return nil, fmt.Errorf("top visitors scan: %w", err)
		}
		visitors = append(visitors, v)
	}
	return visitors, nil
}

// GetVisitorActivity returns detailed page views for a specific IP
func (s *AnalyticsService) GetVisitorActivity(ip string, period string) ([]VisitorDetail, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	rows, err := s.db.Query(`
		SELECT path, viewed_at::text, user_agent, COALESCE(NULLIF(referrer,''),'(direct)')
		FROM page_views WHERE ip_address = $1::inet AND viewed_at >= $2::date
		ORDER BY viewed_at DESC LIMIT 200`, ip, since)
	if err != nil {
		return nil, fmt.Errorf("visitor activity query: %w", err)
	}
	defer rows.Close()

	var details []VisitorDetail
	for rows.Next() {
		var d VisitorDetail
		if err := rows.Scan(&d.Path, &d.ViewedAt, &d.UserAgent, &d.Referrer); err != nil {
			return nil, fmt.Errorf("visitor activity scan: %w", err)
		}
		details = append(details, d)
	}
	return details, nil
}

// GetHourlyActivity returns view counts grouped by hour of day
func (s *AnalyticsService) GetHourlyActivity(period string) ([]HourlyActivity, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	rows, err := s.db.Query(`
		SELECT EXTRACT(HOUR FROM viewed_at)::int AS hour, COUNT(*) AS views
		FROM page_views WHERE viewed_at >= $1::date
		GROUP BY hour ORDER BY hour`, since)
	if err != nil {
		return nil, fmt.Errorf("hourly activity query: %w", err)
	}
	defer rows.Close()

	var activity []HourlyActivity
	for rows.Next() {
		var h HourlyActivity
		if err := rows.Scan(&h.Hour, &h.Views); err != nil {
			return nil, fmt.Errorf("hourly activity scan: %w", err)
		}
		activity = append(activity, h)
	}
	return activity, nil
}

// GetBrowserStats returns browser usage breakdown for the given period
func (s *AnalyticsService) GetBrowserStats(period string) ([]BrowserStat, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	rows, err := s.db.Query(`
		SELECT user_agent, COUNT(*) AS cnt
		FROM page_views WHERE viewed_at >= $1::date
		GROUP BY user_agent`, since)
	if err != nil {
		return nil, fmt.Errorf("browser stats query: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	var total int64
	for rows.Next() {
		var ua string
		var cnt int64
		if err := rows.Scan(&ua, &cnt); err != nil {
			return nil, fmt.Errorf("browser stats scan: %w", err)
		}
		browser := classifyBrowser(ua)
		counts[browser] += cnt
		total += cnt
	}

	var stats []BrowserStat
	for name, count := range counts {
		pct := 0.0
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		stats = append(stats, BrowserStat{Name: name, Count: count, Percent: pct})
	}

	// Sort by count descending
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].Count > stats[i].Count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	return stats, nil
}

func classifyBrowser(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "bot") || strings.Contains(ua, "crawl") || strings.Contains(ua, "spider"):
		return "Bot"
	case strings.Contains(ua, "edg"):
		return "Edge"
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
		return "Chrome"
	case strings.Contains(ua, "firefox"):
		return "Firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		return "Safari"
	case ua == "":
		return "Unknown"
	default:
		return "Other"
	}
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

// GetCrawlerStats returns crawler activity stats grouped by crawler_type for the given period
func (s *AnalyticsService) GetCrawlerStats(period string) ([]CrawlerStat, error) {
	days := parsePeriodDays(period)
	since := time.Now().AddDate(0, 0, -days).Format(dateFormat)

	rows, err := s.db.Query(`
		SELECT crawler_type,
			COUNT(*) AS requests,
			COUNT(DISTINCT path) AS unique_paths,
			MAX(viewed_at)::text AS last_seen
		FROM page_views
		WHERE viewed_at >= $1::date
			AND crawler_type != ''
		GROUP BY crawler_type
		ORDER BY requests DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("crawler stats query: %w", err)
	}
	defer rows.Close()

	var stats []CrawlerStat
	for rows.Next() {
		var cs CrawlerStat
		if err := rows.Scan(&cs.CrawlerType, &cs.Requests, &cs.UniquePaths, &cs.LastSeen); err != nil {
			return nil, fmt.Errorf("crawler stats scan: %w", err)
		}
		stats = append(stats, cs)
	}
	return stats, nil
}

// ClassifyCrawler identifies the crawler type from a User-Agent string.
// Returns an empty string if the User-Agent does not match any known crawler.
func ClassifyCrawler(ua string) string {
	ua = strings.ToLower(ua)
	// Search engines
	if strings.Contains(ua, "googlebot") {
		return "GoogleBot"
	}
	if strings.Contains(ua, "bingbot") {
		return "BingBot"
	}
	if strings.Contains(ua, "yandexbot") {
		return "YandexBot"
	}
	if strings.Contains(ua, "baiduspider") {
		return "BaiduSpider"
	}
	if strings.Contains(ua, "duckduckbot") {
		return "DuckDuckBot"
	}
	// LLM crawlers
	if strings.Contains(ua, "claudebot") || strings.Contains(ua, "anthropic") {
		return "ClaudeBot"
	}
	if strings.Contains(ua, "gptbot") || strings.Contains(ua, "chatgpt") {
		return "GPTBot"
	}
	if strings.Contains(ua, "google-extended") {
		return "Google-Extended"
	}
	if strings.Contains(ua, "perplexitybot") {
		return "PerplexityBot"
	}
	if strings.Contains(ua, "amazonbot") {
		return "AmazonBot"
	}
	if strings.Contains(ua, "cohere-ai") {
		return "CohereBot"
	}
	// Generic bots
	if strings.Contains(ua, "bot") || strings.Contains(ua, "crawl") || strings.Contains(ua, "spider") || strings.Contains(ua, "scrape") {
		return "Other Bot"
	}
	return ""
}
