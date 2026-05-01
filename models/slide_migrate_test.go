package models

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// dbForMigrateTest opens the local dev Postgres (docker container `pg`
// on :5433) for migration tests. Skips when the DB is unreachable so
// `go test ./...` works without docker for unrelated suites.
func dbForMigrateTest(t *testing.T) *sql.DB {
	t.Helper()
	host := envOr("PG_HOST", "127.0.0.1")
	port := envOr("PG_PORT", "5433")
	user := envOr("PG_USER", "blog")
	pass := envOr("PG_PASSWORD", "blogpass")
	name := envOr("PG_DB", "blog")
	dsn := "host=" + host + " port=" + port + " user=" + user + " password=" + pass + " dbname=" + name + " sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	db.SetConnMaxLifetime(5 * time.Second)
	if err := db.Ping(); err != nil {
		t.Skipf("postgres ping: %v", err)
	}
	return db
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// themeIDIs returns true iff the given slide_metadata blob contains
// theme_id == want, tolerant of jsonb whitespace normalization.
func themeIDIs(metadata, want string) bool {
	return strings.Contains(metadata, `"theme_id":"`+want+`"`) ||
		strings.Contains(metadata, `"theme_id": "`+want+`"`)
}

// ensureTestUser creates (or returns) a stable test user the migration
// tests can FK to. The Slides.user_id FK requires it.
func ensureTestUser(t *testing.T, db *sql.DB) int {
	t.Helper()
	var id int
	err := db.QueryRow(`SELECT user_id FROM users WHERE username = 'goslide-test'`).Scan(&id)
	if err == nil {
		return id
	}
	err = db.QueryRow(`INSERT INTO users (username, email, password, full_name, created_at)
		VALUES ('goslide-test', 'goslide-test@example.com', '!', 'GoSlide Test', NOW())
		RETURNING user_id`).Scan(&id)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	return id
}

// TestMigrateThemeID_BackfillsEmptyMetadata verifies the most common case:
// an existing slide whose slide_metadata is the JSONB default '{}' gets
// theme_id set based on whether its content uses .aal-* classes.
func TestMigrateThemeID_BackfillsEmptyMetadata(t *testing.T) {
	db := dbForMigrateTest(t)
	defer db.Close()
	uid := ensureTestUser(t, db)

	// slide_metadata is JSONB — NULL and '{}' are the only "empty"
	// states we'd see in the wild. Test both.
	var aalID, defID int
	err := db.QueryRow(`INSERT INTO Slides (user_id, title, slug, content_file_path, content, slide_metadata, created_at, updated_at)
		VALUES ($1, 'AAL Test', 't-aal-' || extract(epoch from now())::text, '', $2, NULL, NOW(), NOW()) RETURNING slide_id`,
		uid, `<section class="aal pptx-slide"><h1>X</h1></section>`).Scan(&aalID)
	if err != nil {
		t.Fatalf("insert aal: %v", err)
	}
	err = db.QueryRow(`INSERT INTO Slides (user_id, title, slug, content_file_path, content, slide_metadata, created_at, updated_at)
		VALUES ($1, 'Default Test', 't-def-' || extract(epoch from now())::text, '', $2, '{}'::jsonb, NOW(), NOW()) RETURNING slide_id`,
		uid, `<section class="pptx-slide"><h1>Y</h1></section>`).Scan(&defID)
	if err != nil {
		t.Fatalf("insert default: %v", err)
	}
	defer db.Exec(`DELETE FROM Slides WHERE slide_id IN ($1,$2)`, aalID, defID)

	ss := &SlideService{DB: db}
	ss.MigrateThemeID()

	// Verify.
	var meta string
	if err := db.QueryRow(`SELECT slide_metadata FROM Slides WHERE slide_id = $1`, aalID).Scan(&meta); err != nil {
		t.Fatal(err)
	}
	// Postgres jsonb normalizes whitespace — accept either "k":"v" or "k": "v".
	if !themeIDIs(meta, "aal") {
		t.Errorf("aal slide metadata = %q, want theme_id=aal", meta)
	}
	if err := db.QueryRow(`SELECT slide_metadata FROM Slides WHERE slide_id = $1`, defID).Scan(&meta); err != nil {
		t.Fatal(err)
	}
	if !themeIDIs(meta, "default") {
		t.Errorf("default slide metadata = %q, want theme_id=default", meta)
	}
}

// TestMigrateThemeID_SkipsExistingThemeID verifies idempotency: a slide
// already carrying theme_id is left untouched.
func TestMigrateThemeID_SkipsExistingThemeID(t *testing.T) {
	db := dbForMigrateTest(t)
	defer db.Close()
	uid := ensureTestUser(t, db)

	var id int
	err := db.QueryRow(`INSERT INTO Slides (user_id, title, slug, content_file_path, content, slide_metadata, created_at, updated_at)
		VALUES ($1, 'Already Themed', 't-pre-' || extract(epoch from now())::text, '', $2, $3::jsonb, NOW(), NOW()) RETURNING slide_id`,
		uid,
		`<section class="aal pptx-slide"><h1>X</h1></section>`,
		`{"theme_id":"midnight","slideCount":3}`).Scan(&id)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	defer db.Exec(`DELETE FROM Slides WHERE slide_id = $1`, id)

	ss := &SlideService{DB: db}
	ss.MigrateThemeID()

	var meta string
	if err := db.QueryRow(`SELECT slide_metadata FROM Slides WHERE slide_id = $1`, id).Scan(&meta); err != nil {
		t.Fatal(err)
	}
	if !themeIDIs(meta, "midnight") {
		t.Errorf("metadata = %q, want theme_id=midnight preserved", meta)
	}
	// jsonb normalizes whitespace; check for 3 either bare or stringified.
	if !strings.Contains(meta, `"slideCount": 3`) && !strings.Contains(meta, `"slideCount":3`) {
		t.Errorf("metadata = %q, want slideCount preserved", meta)
	}
}
