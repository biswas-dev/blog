package models

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/lib/pq"
)

// PDFHighlight is a text markup annotation (highlight, underline, ...) read from a PDF.
type PDFHighlight struct {
	ID          string          // /NM, or "pdfhash:" + a hash of page and quads when /NM is missing
	Page        int             // 1-based
	Text        string          // page text under the quads, whitespace-collapsed; "" if none found
	Note        string          // /Contents
	Color       string          // "#rrggbb" from /C, or "yellow"
	BoundingBox json.RawMessage // {"rect":[...],"quadpoints":[...]}
}

var markupSubtypes = map[string]bool{"Highlight": true, "Underline": true, "StrikeOut": true, "Squiggly": true}

// ExtractPDFHighlights returns every text markup annotation in the PDF, replies excluded.
// An error means the file could not be read and nothing about its annotations is known.
func ExtractPDFHighlights(data []byte) (hs []PDFHighlight, err error) {
	defer func() {
		if r := recover(); r != nil {
			hs, err = nil, fmt.Errorf("read pdf: %v", r)
		}
	}()
	rd, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	for n := 1; n <= rd.NumPage(); n++ {
		page := rd.Page(n)
		annots := page.V.Key("Annots")
		var pageHs []PDFHighlight
		var quads [][]float64
		for i := 0; i < annots.Len(); i++ {
			a := annots.Index(i)
			if !markupSubtypes[a.Key("Subtype").Name()] || !a.Key("IRT").IsNull() {
				continue
			}
			rect, qp := floats(a.Key("Rect")), floats(a.Key("QuadPoints"))
			if len(qp) < 8 {
				qp = rectQuad(rect)
			}
			box, _ := json.Marshal(map[string][]float64{"rect": rect, "quadpoints": qp})
			id := a.Key("NM").Text()
			if id == "" {
				id = fallbackAnnotID(n, qp)
			}
			pageHs = append(pageHs, PDFHighlight{ID: id, Page: n, Note: a.Key("Contents").Text(), Color: pdfColorHex(a.Key("C")), BoundingBox: box})
			quads = append(quads, qp)
		}
		if len(pageHs) == 0 {
			continue
		}
		glyphs := pageGlyphs(page)
		for i := range pageHs {
			pageHs[i].Text = textInQuads(glyphs, quads[i])
		}
		hs = append(hs, pageHs...)
	}
	return hs, nil
}

// pageGlyphs returns the page's text items, or nil if its content can't be interpreted
// (the library panics on malformed content streams).
func pageGlyphs(page pdf.Page) (glyphs []pdf.Text) {
	defer func() {
		if recover() != nil {
			glyphs = nil
		}
	}()
	for _, t := range page.Content().Text {
		if strings.TrimSpace(t.S) != "" || t.S == " " {
			glyphs = append(glyphs, t)
		}
	}
	return glyphs
}

// textInQuads joins the glyphs whose baseline midpoint lies in one of the quads (each quad is
// 8 numbers, x1 y1 ... x4 y4), ordered by quad, then line (top down), then x.
func textInQuads(glyphs []pdf.Text, qp []float64) string {
	const tol = 1.0
	type hit struct {
		pdf.Text
		quad int
	}
	var hits []hit
	for _, g := range glyphs {
		cx, cy := g.X+g.W/2, g.Y+g.FontSize*0.25
		for q := 0; q+8 <= len(qp); q += 8 {
			minX, maxX := math.Min(math.Min(qp[q], qp[q+2]), math.Min(qp[q+4], qp[q+6])), math.Max(math.Max(qp[q], qp[q+2]), math.Max(qp[q+4], qp[q+6]))
			minY, maxY := math.Min(math.Min(qp[q+1], qp[q+3]), math.Min(qp[q+5], qp[q+7])), math.Max(math.Max(qp[q+1], qp[q+3]), math.Max(qp[q+5], qp[q+7]))
			if cx >= minX-tol && cx <= maxX+tol && cy >= minY-tol && cy <= maxY+tol {
				hits = append(hits, hit{g, q})
				break
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.quad != b.quad {
			return a.quad < b.quad
		}
		if sameLine(a.Text, b.Text) {
			return a.X < b.X
		}
		return a.Y > b.Y
	})
	var sb strings.Builder
	for i, h := range hits {
		if i > 0 {
			prev := hits[i-1]
			// Papers rarely contain space glyphs; word gaps come from positioning.
			if prev.quad != h.quad || !sameLine(prev.Text, h.Text) || h.X-(prev.X+prev.W) > 0.2*h.FontSize {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(h.S)
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

func sameLine(a, b pdf.Text) bool {
	return math.Abs(a.Y-b.Y) < math.Max(a.FontSize, b.FontSize)/2
}

func floats(v pdf.Value) []float64 {
	out := make([]float64, v.Len())
	for i := range out {
		out[i] = v.Index(i).Float64()
	}
	return out
}

// rectQuad turns a /Rect [x1 y1 x2 y2] into one quad for annotations without /QuadPoints.
func rectQuad(r []float64) []float64 {
	if len(r) < 4 {
		return nil
	}
	return []float64{r[0], r[3], r[2], r[3], r[0], r[1], r[2], r[1]}
}

func fallbackAnnotID(page int, qp []float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d", page)
	for _, f := range qp {
		fmt.Fprintf(&sb, ",%.2f", f)
	}
	sum := sha1.Sum([]byte(sb.String()))
	return "pdfhash:" + hex.EncodeToString(sum[:12])
}

func pdfColorHex(c pdf.Value) string {
	if c.Len() != 3 {
		return "yellow"
	}
	b := func(i int) int { return int(math.Round(math.Max(0, math.Min(1, c.Index(i).Float64())) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", b(0), b(1), b(2))
}

// PDFSyncResult counts what SyncFromPDF changed.
type PDFSyncResult struct {
	Upserted, SkippedEmpty, Deleted int
}

// SyncFromPDF makes the paper's PDF-sourced annotations (pdf_annot_id set) match hs: upserts
// those with text (new rows public, authored by authorID; existing rows keep is_public and
// sort_order) and deletes those no longer in the PDF. Highlights with no extracted text are
// skipped but kept. Rows without pdf_annot_id (pre-PdfCraft) are never touched.
func (s *PaperAnnotationService) SyncFromPDF(paperID, authorID int, hs []PDFHighlight) (PDFSyncResult, error) {
	var res PDFSyncResult
	tx, err := s.DB.Begin()
	if err != nil {
		return res, fmt.Errorf("begin pdf sync: %w", err)
	}
	defer tx.Rollback()

	present := []string{} // non-nil: pq.Array(nil) is NULL and "<> ALL(NULL)" would delete nothing
	for _, h := range hs {
		present = append(present, h.ID)
		if h.Text == "" {
			res.SkippedEmpty++
			continue
		}
		_, err := tx.Exec(`
			INSERT INTO paper_annotations (paper_id, author_id, page_number, selected_text, bounding_box, color, note, is_public, pdf_annot_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8)
			ON CONFLICT (paper_id, pdf_annot_id) WHERE pdf_annot_id IS NOT NULL DO UPDATE
			SET page_number = EXCLUDED.page_number, selected_text = EXCLUDED.selected_text,
			    bounding_box = EXCLUDED.bounding_box, color = EXCLUDED.color, note = EXCLUDED.note, updated_at = NOW()`,
			paperID, authorID, h.Page, h.Text, []byte(h.BoundingBox), h.Color, h.Note, h.ID)
		if err != nil {
			return res, fmt.Errorf("upsert pdf annotation %q: %w", h.ID, err)
		}
		res.Upserted++
	}
	r, err := tx.Exec(`DELETE FROM paper_annotations WHERE paper_id = $1 AND pdf_annot_id IS NOT NULL AND pdf_annot_id <> ALL($2)`,
		paperID, pq.Array(present))
	if err != nil {
		return res, fmt.Errorf("delete removed pdf annotations: %w", err)
	}
	n, _ := r.RowsAffected()
	res.Deleted = int(n)
	return res, tx.Commit()
}
