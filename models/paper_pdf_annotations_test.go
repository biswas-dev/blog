package models

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// buildPDF writes a minimal PDF with the given object bodies (object 1 must be the catalog).
func buildPDF(objs []string) []byte {
	var sb strings.Builder
	sb.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = sb.Len()
		fmt.Fprintf(&sb, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := sb.Len()
	fmt.Fprintf(&sb, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&sb, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&sb, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(sb.String())
}

// testPaperPDF: a monospaced font (600/1000 em, so 7.2pt per glyph at 12pt) and two lines
// whose words are separated by TJ offsets, not space glyphs, as in LaTeX output.
//
//	line 1, baseline 100: Hello 20-56, brave 59.6-95.6, new 99.2-120.8, world 124.4-160.4
//	line 2, baseline 80:  second 20-63.2, line 66.8-95.6
func testPaperPDF() []byte {
	content := "BT /F1 12 Tf 20 100 Td [(Hello) -300 (brave) -300 (new) -300 (world)] TJ ET\n" +
		"BT /F1 12 Tf 20 80 Td [(second) -300 (line)] TJ ET\n"
	widths := strings.TrimSpace(strings.Repeat("600 ", 95))
	return buildPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R /Annots [6 0 R 7 0 R 8 0 R 9 0 R] >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 32 /LastChar 126 /Widths [" + widths + "] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		"<< /Type /Annot /Subtype /Highlight /Rect [58 97 122 110] /QuadPoints [58 110 122 110 58 97 122 97] /C [1 0.94 0] /T (Me) /NM (hl-a) /Contents (Great point) /P 3 0 R >>",
		"<< /Type /Annot /Subtype /Underline /Rect [18 77 162 110] /QuadPoints [123 110 162 110 123 97 162 97 18 90 64 90 18 77 64 77] /C [0 0 1] /P 3 0 R >>",
		"<< /Type /Annot /Subtype /Highlight /Rect [150 150 190 170] /QuadPoints [150 170 190 170 150 150 190 150] /NM (hl-empty) /P 3 0 R >>",
		"<< /Type /Annot /Subtype /Text /Rect [58 97 122 110] /IRT 6 0 R /NM (reply) /Contents (A reply) /P 3 0 R >>",
	})
}

func TestExtractPDFHighlights(t *testing.T) {
	hs, err := ExtractPDFHighlights(testPaperPDF())
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 3 {
		t.Fatalf("got %d highlights, want 3 (reply excluded): %+v", len(hs), hs)
	}
	a, b, empty := hs[0], hs[1], hs[2]
	if a.ID != "hl-a" || a.Page != 1 || a.Text != "brave new" || a.Note != "Great point" || a.Color != "#fff000" {
		t.Errorf("highlight a = %+v", a)
	}
	if !strings.Contains(string(a.BoundingBox), `"quadpoints":[58,110,122,110,58,97,122,97]`) {
		t.Errorf("bounding box = %s", a.BoundingBox)
	}
	if !strings.HasPrefix(b.ID, "pdfhash:") || b.Text != "world second" || b.Color != "#0000ff" {
		t.Errorf("highlight b (no /NM, two quads) = %+v", b)
	}
	if again, _ := ExtractPDFHighlights(testPaperPDF()); again[1].ID != b.ID {
		t.Errorf("fallback id not stable: %q vs %q", again[1].ID, b.ID)
	}
	if empty.ID != "hl-empty" || empty.Text != "" {
		t.Errorf("empty highlight = %+v", empty)
	}
}

func TestExtractPDFHighlightsRejectsGarbage(t *testing.T) {
	if _, err := ExtractPDFHighlights([]byte("%PDF-1.4\nnot really")); err == nil {
		t.Fatal("want error for unreadable pdf")
	}
}

func TestSyncFromPDF(t *testing.T) {
	db := SetupTestDB(t)
	suffix := fmt.Sprint(time.Now().UnixNano())
	userID := SeedUser(t, db, "kp"+suffix+"@example.com", "kp"+suffix, "password123", RoleAdministrator)
	t.Cleanup(func() { CleanupUser(t, db, userID) })
	var paperID int
	if err := db.QueryRow(`INSERT INTO papers (user_id, title, slug) VALUES ($1, 'KP', $2) RETURNING paper_id`, userID, "kp-"+suffix).Scan(&paperID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM papers WHERE paper_id = $1`, paperID) })

	s := &PaperAnnotationService{DB: db}
	legacy, err := s.Create(paperID, userID, 2, "legacy text", []byte(`{}`), "yellow", "", true)
	if err != nil {
		t.Fatal(err)
	}
	byID := func() map[string]PaperAnnotation {
		t.Helper()
		rows, err := db.Query(`SELECT COALESCE(pdf_annot_id, ''), annotation_id, selected_text, note, color, is_public, page_number FROM paper_annotations WHERE paper_id = $1`, paperID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		m := map[string]PaperAnnotation{}
		for rows.Next() {
			var k string
			var a PaperAnnotation
			if err := rows.Scan(&k, &a.ID, &a.SelectedText, &a.Note, &a.Color, &a.IsPublic, &a.PageNumber); err != nil {
				t.Fatal(err)
			}
			m[k] = a
		}
		return m
	}
	box := []byte(`{"rect":[0,0,1,1]}`)
	a := PDFHighlight{ID: "a", Page: 1, Text: "alpha", Note: "n1", Color: "#ff0000", BoundingBox: box}
	b := PDFHighlight{ID: "b", Page: 3, Text: "beta", BoundingBox: box}
	c := PDFHighlight{ID: "c", Page: 1, Text: "", BoundingBox: box}

	res, err := s.SyncFromPDF(paperID, userID, []PDFHighlight{a, b, c})
	if err != nil || res != (PDFSyncResult{Upserted: 2, SkippedEmpty: 1}) {
		t.Fatalf("first sync = %+v, %v", res, err)
	}
	m := byID()
	if len(m) != 3 || !m["a"].IsPublic || m["a"].SelectedText != "alpha" || m["a"].Color != "#ff0000" || m["b"].PageNumber != 3 {
		t.Fatalf("after first sync: %+v", m)
	}

	// The admin hides "a"; a later save edits its note and must keep it hidden.
	idA := m["a"].ID
	if err := s.Update(idA, "#ff0000", "n1", false, 0); err != nil {
		t.Fatal(err)
	}
	a.Note = "n2"
	b.Text = "" // text no longer extractable: keep the row as it was
	if res, err = s.SyncFromPDF(paperID, userID, []PDFHighlight{a, b}); err != nil || res.Deleted != 0 {
		t.Fatalf("second sync = %+v, %v", res, err)
	}
	m = byID()
	if m["a"].IsPublic || m["a"].Note != "n2" || m["a"].ID != idA || m["b"].SelectedText != "beta" {
		t.Fatalf("after second sync: %+v", m)
	}

	// Every highlight removed from the PDF: synced rows go, the legacy row stays.
	if res, err = s.SyncFromPDF(paperID, userID, nil); err != nil || res.Deleted != 2 {
		t.Fatalf("third sync = %+v, %v", res, err)
	}
	m = byID()
	if len(m) != 1 || m[""].ID != legacy.ID {
		t.Fatalf("after third sync: %+v", m)
	}
}
