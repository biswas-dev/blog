package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
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
	if _, err := os.Stat(paper.PDFFileURL); os.IsNotExist(err) {
		log.Printf("PDF file not found on disk: %s", paper.PDFFileURL)
		http.Error(w, "PDF file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, paper.PDFFileURL)
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
