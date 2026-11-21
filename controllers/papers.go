package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/go-chi/chi/v5"
)

// Papers handles paper CRUD, PDF serving, and annotation API.
type Papers struct {
	Templates struct {
		PapersList  views.Template
		PaperPage   views.Template
		AdminPapers views.Template
		PaperEditor views.Template
	}
	PaperService           *models.PaperService
	PaperVersionService    *models.PaperVersionService
	PaperAnnotationService *models.PaperAnnotationService
	SessionService         *models.SessionService
	ResearchAreaService    *models.PaperResearchAreaService
	BlogWiki               *gowiki.Wiki
}

// PublicPapersList displays the public papers listing page.
// GET /papers
func (p Papers) PublicPapersList(w http.ResponseWriter, r *http.Request) {
	user, _ := utils.IsUserLoggedIn(r, p.SessionService)

	papersList, err := p.PaperService.GetPublishedPapers()
	if err != nil {
		log.Printf("Error fetching papers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Papers          []models.Paper
		UserPermissions models.UserPermissions
		FilterName      string
		FilterType      string
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Research paper reviews and notes - Anshuman Biswas Blog",
		CurrentPage:     "papers",
		Papers:          papersList.Papers,
		UserPermissions: userPerms,
	}

	p.Templates.PapersList.Execute(w, r, data)
}

// ViewPaper displays a single paper with content and public annotations.
// GET /papers/{slug}
func (p Papers) ViewPaper(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user, _ := utils.IsUserLoggedIn(r, p.SessionService)

	paper, err := p.PaperService.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check if paper is published (unless user can view unpublished)
	if !paper.IsPublished && (user == nil || !models.CanViewUnpublished(user.Role)) {
		http.NotFound(w, r)
		return
	}

	// Load public annotations for "Key Passages" section
	publicAnnotations, err := p.PaperAnnotationService.GetPublicByPaper(paper.ID)
	if err != nil {
		log.Printf("Error fetching public annotations for paper %d: %v", paper.ID, err)
		publicAnnotations = nil
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	// Compute full URL for share links
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s/papers/%s", scheme, r.Host, slug)

	data := struct {
		LoggedIn          bool
		Username          string
		IsAdmin           bool
		SignupDisabled    bool
		UserID            int
		Description       string
		CurrentPage       string
		Paper             *models.Paper
		PublicAnnotations []models.PaperAnnotation
		FullURL           string
		UserPermissions   models.UserPermissions
	}{
		LoggedIn:          user != nil,
		Username:          getUsername(user),
		IsAdmin:           user != nil && models.IsAdmin(user.Role),
		SignupDisabled:    true,
		Description:       paper.Title,
		CurrentPage:       "papers",
		Paper:             paper,
		PublicAnnotations: publicAnnotations,
		FullURL:           fullURL,
		UserPermissions:   userPerms,
	}

	if user != nil {
		data.UserID = user.UserID
	}

	p.Templates.PaperPage.Execute(w, r, data)
}

// AreaPage displays papers filtered by a specific research area.
// GET /papers/area/{name}
func (p Papers) AreaPage(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "name")
	name, _ := url.PathUnescape(raw)
	name = strings.ReplaceAll(name, "+", " ") // + is space in URLs
	if name == "" {
		http.NotFound(w, r)
		return
	}

	user, _ := utils.IsUserLoggedIn(r, p.SessionService)

	area, err := p.ResearchAreaService.GetByName(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	papers, err := p.PaperService.GetPublishedPapersByArea(area.ID)
	if err != nil {
		log.Printf("Error fetching papers by area: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		AreaName        string
		Papers          []models.Paper
		UserPermissions models.UserPermissions
		FilterName      string
		FilterType      string
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     fmt.Sprintf("Papers in %s - Anshuman Biswas Blog", area.Name),
		CurrentPage:     "papers",
		AreaName:        area.Name,
		Papers:          papers,
		UserPermissions: userPerms,
		FilterName:      area.Name,
		FilterType:      "area",
	}

	p.Templates.PapersList.Execute(w, r, data)
}

// AdminPapers displays the admin papers management page.
// GET /admin/papers
func (p Papers) AdminPapers(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	papersList, err := p.PaperService.GetAllPapers()
	if err != nil {
		log.Printf("Error fetching papers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	areas, err := p.ResearchAreaService.GetAll()
	if err != nil {
		log.Printf("Error fetching research areas: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Papers          []models.Paper
		ResearchAreas   []models.PaperResearchArea
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Manage Papers - Anshuman Biswas Blog",
		CurrentPage:     "admin-papers",
		Papers:          papersList.Papers,
		ResearchAreas:   areas,
		UserPermissions: models.GetPermissions(user.Role),
	}

	p.Templates.AdminPapers.Execute(w, r, data)
}

// NewPaper displays the paper creation editor.
// GET /admin/papers/new
func (p Papers) NewPaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	areas, err := p.ResearchAreaService.GetAll()
	if err != nil {
		log.Printf("Error fetching research areas: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate editor HTML from go-wiki
	editorHTML, err := p.BlogWiki.EditorHTML("")
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		ResearchAreas   []models.PaperResearchArea
		SelectedAreaMap map[int]bool
		Paper           *models.Paper
		Mode            string
		EditorHTML      template.HTML
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Create New Paper - Anshuman Biswas Blog",
		CurrentPage:     "admin-papers",
		ResearchAreas:   areas,
		SelectedAreaMap: map[int]bool{},
		Paper:           &models.Paper{},
		Mode:            "new",
		EditorHTML:      editorHTML,
		UserPermissions: models.GetPermissions(user.Role),
	}

	p.Templates.PaperEditor.Execute(w, r, data)
}

// CreatePaper handles paper creation.
// POST /admin/papers
func (p Papers) CreatePaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	paperAuthors := r.FormValue("paper_authors")
	abstract := r.FormValue("abstract")
	paperYearStr := r.FormValue("paper_year")
	conference := r.FormValue("conference")
	doi := r.FormValue("doi")
	arxivID := r.FormValue("arxiv_id")
	pdfFileURL := r.FormValue("pdf_file_url")
	pdfFileSizeStr := r.FormValue("pdf_file_size")
	coverImageURL := r.FormValue("cover_image_url")
	content := r.FormValue("content")
	description := r.FormValue("description")
	myNotes := r.FormValue("my_notes")
	ratingStr := r.FormValue("rating")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	areasStr := r.FormValue("areas")

	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	paperYear, _ := strconv.Atoi(paperYearStr)
	pdfFileSize, _ := strconv.ParseInt(pdfFileSizeStr, 10, 64)
	rating, _ := strconv.ParseFloat(ratingStr, 64)

	var areaIDs []int
	if areasStr != "" {
		for _, aStr := range strings.Split(areasStr, ",") {
			if aID, err := strconv.Atoi(strings.TrimSpace(aStr)); err == nil {
				areaIDs = append(areaIDs, aID)
			}
		}
	}

	paper, err := p.PaperService.Create(user.UserID, title, slug, paperAuthors, abstract, paperYear, conference, doi, arxivID, pdfFileURL, pdfFileSize, coverImageURL, content, description, myNotes, rating, isPublished, areaIDs)
	if err != nil {
		log.Printf("Error creating paper: %v", err)
		http.Error(w, "Failed to create paper", http.StatusInternalServerError)
		return
	}

	_ = p.PaperVersionService.MaybeCreateVersion(paper.ID, user.UserID, title, content)

	http.Redirect(w, r, fmt.Sprintf("/admin/papers/%d/edit", paper.ID), http.StatusFound)
}

// EditPaper displays the paper editing page.
// GET /admin/papers/{paperID}/edit
func (p Papers) EditPaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, "Invalid paper ID", http.StatusBadRequest)
		return
	}

	paper, err := p.PaperService.GetByID(paperID)
	if err != nil {
		log.Printf("Error fetching paper: %v", err)
		http.Error(w, "Paper not found", http.StatusNotFound)
		return
	}

	areas, err := p.ResearchAreaService.GetAll()
	if err != nil {
		log.Printf("Error fetching research areas: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert paper research areas to selected area map
	selectedAreaMap := make(map[int]bool)
	for _, area := range paper.ResearchAreas {
		selectedAreaMap[area.ID] = true
	}

	// Generate editor HTML from go-wiki
	editorHTML, err := p.BlogWiki.EditorHTML(paper.Content)
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
	}

	// Existing highlights (pre-PdfCraft) still feed the public Key Passages; list them for managing.
	annotations, err := p.PaperAnnotationService.GetByPaper(paper.ID)
	if err != nil {
		log.Printf("Error fetching annotations for paper %d: %v", paper.ID, err)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		ResearchAreas   []models.PaperResearchArea
		SelectedAreaMap map[int]bool
		Paper           *models.Paper
		Mode            string
		EditorHTML      template.HTML
		Annotations     []models.PaperAnnotation
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Annotations:     annotations,
		Description:     "Edit Paper - Anshuman Biswas Blog",
		CurrentPage:     "admin-papers",
		ResearchAreas:   areas,
		SelectedAreaMap: selectedAreaMap,
		Paper:           paper,
		Mode:            "edit",
		EditorHTML:      editorHTML,
		UserPermissions: models.GetPermissions(user.Role),
	}

	p.Templates.PaperEditor.Execute(w, r, data)
}

// UpdatePaper handles paper updates.
// POST /admin/papers/{paperID}
func (p Papers) UpdatePaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, "Invalid paper ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	paperAuthors := r.FormValue("paper_authors")
	abstract := r.FormValue("abstract")
	paperYearStr := r.FormValue("paper_year")
	conference := r.FormValue("conference")
	doi := r.FormValue("doi")
	arxivID := r.FormValue("arxiv_id")
	pdfFileURL := r.FormValue("pdf_file_url")
	pdfFileSizeStr := r.FormValue("pdf_file_size")
	coverImageURL := r.FormValue("cover_image_url")
	content := r.FormValue("content")
	description := r.FormValue("description")
	myNotes := r.FormValue("my_notes")
	ratingStr := r.FormValue("rating")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	areasStr := r.FormValue("areas")

	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	paperYear, _ := strconv.Atoi(paperYearStr)
	pdfFileSize, _ := strconv.ParseInt(pdfFileSizeStr, 10, 64)
	rating, _ := strconv.ParseFloat(ratingStr, 64)

	var areaIDs []int
	if areasStr != "" {
		for _, aStr := range strings.Split(areasStr, ",") {
			if aID, err := strconv.Atoi(strings.TrimSpace(aStr)); err == nil {
				areaIDs = append(areaIDs, aID)
			}
		}
	}

	err = p.PaperService.Update(paperID, title, slug, paperAuthors, abstract, paperYear, conference, doi, arxivID, pdfFileURL, pdfFileSize, coverImageURL, content, description, myNotes, rating, isPublished, areaIDs)
	if err != nil {
		log.Printf("Error updating paper: %v", err)
		http.Error(w, "Failed to update paper", http.StatusInternalServerError)
		return
	}

	_ = p.PaperVersionService.MaybeCreateVersion(paperID, user.UserID, title, content)

	http.Redirect(w, r, fmt.Sprintf("/admin/papers/%d/edit", paperID), http.StatusFound)
}

// DeletePaper handles paper deletion.
// POST /admin/papers/{paperID}/delete
func (p Papers) DeletePaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, "Invalid paper ID", http.StatusBadRequest)
		return
	}

	err = p.PaperService.Delete(paperID)
	if err != nil {
		log.Printf("Error deleting paper: %v", err)
		http.Error(w, "Failed to delete paper", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/papers", http.StatusFound)
}

// PreviewPaper handles markdown preview for papers.
// POST /admin/papers/preview
func (p Papers) PreviewPaper(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	rendered := models.RenderContent(content)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, template.HTML(rendered))
}

// ServePDF serves the PDF file for a paper (admin/editor only).
// GET /admin/papers/{paperID}/pdf
func (p Papers) ServePDF(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, "Invalid paper ID", http.StatusBadRequest)
		return
	}

	paper, err := p.PaperService.GetByID(paperID)
	if err != nil {
		log.Printf("Error fetching paper: %v", err)
		http.Error(w, "Paper not found", http.StatusNotFound)
		return
	}

	if paper.PDFFileURL == "" {
		http.Error(w, "No PDF available", http.StatusNotFound)
		return
	}

	// Verify file exists
	pdfPath, ok := paperPDFPath(paper.PDFFileURL)
	if _, err := os.Stat(pdfPath); !ok || err != nil {
		log.Printf("PDF file not found on disk: %s", paper.PDFFileURL)
		http.Error(w, "PDF file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeFile(w, r, pdfPath)
}

// maxPaperPDFBytes caps a PDF saved back from PdfCraft.
const maxPaperPDFBytes = 100 << 20

// SavePDF replaces a paper's PDF with what PdfCraft saved (the paper's markup lives in the
// file). The first save keeps the original upload next to it as <name>.orig.pdf.
// POST /admin/papers/{paperID}/pdf, body = the PDF.
func (p Papers) SavePDF(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil || !models.GetPermissions(user.Role).CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	paperID, err := strconv.Atoi(chi.URLParam(r, "paperID"))
	if err != nil {
		http.Error(w, "Invalid paper ID", http.StatusBadRequest)
		return
	}
	paper, err := p.PaperService.GetByID(paperID)
	if err != nil {
		http.Error(w, "Paper not found", http.StatusNotFound)
		return
	}
	pdfPath, ok := paperPDFPath(paper.PDFFileURL)
	if !ok {
		http.Error(w, "Paper has no PDF", http.StatusNotFound)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPaperPDFBytes))
	if err != nil {
		http.Error(w, "PDF is too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		http.Error(w, "Not a PDF", http.StatusBadRequest)
		return
	}
	if err := replacePaperPDF(pdfPath, data); err != nil {
		log.Printf("SavePDF %d: %v", paperID, err)
		http.Error(w, "Failed to save PDF", http.StatusInternalServerError)
		return
	}
	if err := p.PaperService.SetPDFSize(paperID, int64(len(data))); err != nil {
		log.Printf("SavePDF %d: size: %v", paperID, err)
	}
	p.syncPDFAnnotations(paperID, user.UserID, data)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "size": len(data)})
}

// syncPDFAnnotations mirrors the saved PDF's highlights into Key Passages. The PDF is already
// stored, so failures are only logged.
func (p Papers) syncPDFAnnotations(paperID, userID int, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SavePDF %d: annotation sync panic: %v", paperID, r)
		}
	}()
	hs, err := models.ExtractPDFHighlights(data)
	if err != nil {
		log.Printf("SavePDF %d: annotation sync: %v", paperID, err)
		return
	}
	res, err := p.PaperAnnotationService.SyncFromPDF(paperID, userID, hs)
	if err != nil {
		log.Printf("SavePDF %d: annotation sync: %v", paperID, err)
		return
	}
	log.Printf("SavePDF %d: annotation sync: %d upserted, %d deleted, %d skipped (no text)", paperID, res.Upserted, res.Deleted, res.SkippedEmpty)
}

// paperPDFPath maps a stored /static/uploads/papers/... URL to its file, relative to the
// working directory (the image runs from /, so this is /static/... there too).
func paperPDFPath(url string) (string, bool) {
	rel := filepath.Clean(strings.TrimPrefix(url, "/"))
	return rel, strings.HasPrefix(rel, filepath.Join("static", "uploads", "papers")+string(filepath.Separator)) && strings.EqualFold(filepath.Ext(rel), ".pdf")
}

// replacePaperPDF writes data over pdfPath atomically, keeping the first original as .orig.pdf.
func replacePaperPDF(pdfPath string, data []byte) error {
	orig := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + ".orig.pdf"
	if _, err := os.Stat(orig); os.IsNotExist(err) {
		if err := os.Link(pdfPath, orig); err != nil {
			return fmt.Errorf("keep original: %w", err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(pdfPath), ".save-*.pdf")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if fi, err := os.Stat(pdfPath); err == nil {
		_ = tmp.Chmod(fi.Mode().Perm()) // CreateTemp makes 0600; keep the upload's mode
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), pdfPath)
}

// ListAnnotations returns all annotations for a paper as JSON.
// GET /api/papers/{paperID}/annotations
func (p Papers) ListAnnotations(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid paper ID"}`, http.StatusBadRequest)
		return
	}

	annotations, err := p.PaperAnnotationService.GetByPaper(paperID)
	if err != nil {
		log.Printf("Error fetching annotations: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(annotations)
}

// CreateAnnotation creates a new annotation for a paper.
// POST /api/papers/{paperID}/annotations
func (p Papers) CreateAnnotation(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid paper ID"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		PageNumber   int             `json:"page_number"`
		SelectedText string          `json:"selected_text"`
		BoundingBox  json.RawMessage `json:"bounding_box"`
		Color        string          `json:"color"`
		Note         string          `json:"note"`
		IsPublic     bool            `json:"is_public"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	ann, err := p.PaperAnnotationService.Create(paperID, user.UserID, body.PageNumber, body.SelectedText, body.BoundingBox, body.Color, body.Note, body.IsPublic)
	if err != nil {
		log.Printf("Error creating annotation: %v", err)
		http.Error(w, `{"error":"failed to create annotation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ann)
}

// UpdateAnnotation updates an existing annotation.
// PUT /api/papers/annotations/{annotationID}
func (p Papers) UpdateAnnotation(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	annotationIDStr := chi.URLParam(r, "annotationID")
	annotationID, err := strconv.Atoi(annotationIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid annotation ID"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Color     string `json:"color"`
		Note      string `json:"note"`
		IsPublic  bool   `json:"is_public"`
		SortOrder int    `json:"sort_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	err = p.PaperAnnotationService.Update(annotationID, body.Color, body.Note, body.IsPublic, body.SortOrder)
	if err != nil {
		log.Printf("Error updating annotation: %v", err)
		http.Error(w, `{"error":"failed to update annotation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"success":true}`)
}

// DeleteAnnotation removes an annotation.
// DELETE /api/papers/annotations/{annotationID}
func (p Papers) DeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	annotationIDStr := chi.URLParam(r, "annotationID")
	annotationID, err := strconv.Atoi(annotationIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid annotation ID"}`, http.StatusBadRequest)
		return
	}

	err = p.PaperAnnotationService.Delete(annotationID)
	if err != nil {
		log.Printf("Error deleting annotation: %v", err)
		http.Error(w, `{"error":"failed to delete annotation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"success":true}`)
}

// ReorderAnnotations reorders annotations for a paper.
// POST /api/papers/{paperID}/annotations/reorder
func (p Papers) ReorderAnnotations(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, p.SessionService)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	paperIDStr := chi.URLParam(r, "paperID")
	paperID, err := strconv.Atoi(paperIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid paper ID"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		AnnotationIDs []int `json:"annotation_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	err = p.PaperAnnotationService.ReorderAnnotations(paperID, body.AnnotationIDs)
	if err != nil {
		log.Printf("Error reordering annotations: %v", err)
		http.Error(w, `{"error":"failed to reorder annotations"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"success":true}`)
}

// groupPapersByYear organizes papers by their PaperYear field.
func groupPapersByYear(papers []models.Paper) map[int][]models.Paper {
	m := make(map[int][]models.Paper)
	for _, paper := range papers {
		m[paper.PaperYear] = append(m[paper.PaperYear], paper)
	}
	return m
}
