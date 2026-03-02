// Path: main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"anshumanbiswas.com/blog/controllers"
	authmw "anshumanbiswas.com/blog/middleware"
	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/templates"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	godraw "github.com/anchoo2kewl/go-draw"
	godrawstore "github.com/anchoo2kewl/go-draw/store"
	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func getAppPort() string {
	port := os.Getenv("APP_PORT")
	if port == "" {
		return "3000"
	}
	return port
}

func main() {
	// Track application start time for uptime calculation
	startTime := time.Now()

	initLogger()

	apiToken := os.Getenv("API_TOKEN")

	if apiToken == "" {
		logger.Fatal().Msg("API token not set in environment variable: API_TOKEN")
	} else {
		logger.Info().Msg("API token loaded")
	}

	listenAddr := flag.String("listen-addr", ":"+getAppPort(), "server listen address")
	flag.Parse()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Compress(5, "text/html", "text/css", "application/javascript", "application/json", "image/svg+xml"))

	dbUser, dbPassword, dbName, dbHost, dbPort :=
		os.Getenv("PG_USER"),
		os.Getenv("PG_PASSWORD"),
		os.Getenv("PG_DB"),
		os.Getenv("PG_HOST"),
		os.Getenv("PG_PORT")

	database, err := Initialize(dbUser, dbPassword, dbName, dbHost, dbPort)

	if err != nil {
		logger.Fatal().Err(err).Msg("could not set up database")
	}
	defer database.Conn.Close()

	// Initialize AnalyticsService (background goroutines for batch inserts + aggregation)
	analyticsService := models.NewAnalyticsService(DB)
	defer analyticsService.Shutdown()

	userService := models.UserService{
		DB: DB,
	}

	sessionService := models.SessionService{
		DB: DB,
	}

	apiTokenService := models.APITokenService{
		DB: DB,
	}

	// Page view tracking middleware (records after response, zero latency)
	r.Use(authmw.TrackingMiddleware(analyticsService, &sessionService))

	r.Get("/about", controllers.StaticHandler(
		views.Must(views.ParseFS(templates.FS, "about.gohtml", "tailwind.gohtml")), &sessionService))

	// Public docs routes to the formatting guide
	r.Get("/docs/formatting-guide", controllers.StaticHandler(
		views.Must(views.ParseFS(templates.FS, "admin-formatting-guide.gohtml", "tailwind.gohtml")), &sessionService))
	r.Get("/docs/complete-formatting-guide", controllers.StaticHandler(
		views.Must(views.ParseFS(templates.FS, "admin-formatting-guide.gohtml", "tailwind.gohtml")), &sessionService))

	r.Get("/admin/formatting-guide", controllers.StaticHandler(
		views.Must(views.ParseFS(templates.FS, "admin-formatting-guide.gohtml", "tailwind.gohtml")), &sessionService))

	postService := models.PostService{
		DB: DB,
	}

	// Initialize BlogService
	blogService := models.NewBlogService(DB)

	// Initialize CategoryService
	categoryService := models.CategoryService{
		DB: DB,
	}

	// Initialize SlideService
	slideService := models.SlideService{
		DB: DB,
	}

	// Initialize SearchService
	searchService := &models.SearchService{
		DB: DB,
	}
	searchService.BackfillSlideContent()

	// Initialize SystemService
	systemService := models.NewSystemService(DB, "migrations", startTime)

	// Initialize DatabaseBackupService
	databaseBackupService := models.NewDatabaseBackupService(DB)

	// Initialize CloudinaryService
	cloudinaryService := models.CloudinaryService{
		DB: DB,
	}

	// Initialize ImageMetadataService
	imageMetadataService := models.ImageMetadataService{
		DB: DB,
	}

	// Resolve Cloudinary cloud name at startup (empty string if not configured)
	var cloudinaryCloudName string
	if cloudinaryService.IsConfigured() {
		if cs, err := cloudinaryService.Get(); err == nil && cs != nil {
			cloudinaryCloudName = cs.CloudName
		}
	}

	// Blog wiki instance configured for the post editor
	blogWikiOpts := []gowiki.Option{
		gowiki.WithPreviewEndpoint("/admin/preview"),
		gowiki.WithUploadEndpoint("/admin/uploads"),
		gowiki.WithImageListEndpoint("/api/admin/images"),
		gowiki.WithImageMetadataEndpoint("/api/admin/image-metadata"),
		gowiki.WithCloudinarySignatureEndpoint("/api/admin/cloudinary/signature"),
		gowiki.WithDrawBasePath("/draw"),
		gowiki.WithEnableMore(true),
	}
	if cloudinaryCloudName != "" {
		blogWikiOpts = append(blogWikiOpts, gowiki.WithCloudinaryCloudName(cloudinaryCloudName))
	}
	blogWiki := gowiki.New(blogWikiOpts...)

	// Setup our controllers
	usersC := controllers.Users{
		UserService:          &userService,
		SessionService:       &sessionService,
		PostService:          &postService,
		APITokenService:      &apiTokenService,
		CategoryService:      &categoryService,
		CloudinaryService:    &cloudinaryService,
		ImageMetadataService: &imageMetadataService,
		BlogWiki:             blogWiki,
	}

	// Initialize Blog controller
	blogC := controllers.Blog{
		BlogService:    blogService,
		SessionService: &sessionService,
	}

	// Initialize Categories controller
	categoriesC := controllers.Categories{
		CategoryService: &categoryService,
		SessionService:  &sessionService,
	}

	// Initialize Slides controller
	slidesC := controllers.Slides{
		SlideService:    &slideService,
		SessionService:  &sessionService,
		CategoryService: &categoryService,
	}

	// Initialize ExternalSystemService and SyncClient
	externalSystemService := models.ExternalSystemService{
		DB: DB,
	}

	syncClient := models.SyncClient{
		PostService:           &postService,
		CategoryService:       &categoryService,
		ExternalSystemService: &externalSystemService,
	}

	// Initialize Search controller
	searchC := controllers.Search{
		SearchService: searchService,
	}

	// Initialize System controller
	systemC := controllers.System{
		SystemService:         systemService,
		DatabaseBackupService: databaseBackupService,
		SessionService:        &sessionService,
		ExternalSystemService: &externalSystemService,
		SyncClient:            &syncClient,
		CloudinaryService:     &cloudinaryService,
	}

	usersC.Templates.New = views.Must(views.ParseFS(
		templates.FS, "signup.gohtml", "tailwind.gohtml"))

	isSignupDisabled, _ := strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))

	if isSignupDisabled {
		logger.Info().Msg("signups disabled")
		r.Get("/signup", usersC.Disabled)
	} else {
		logger.Info().Msg("signups enabled")
		r.Get("/signup", usersC.New)
		r.Post("/signup", usersC.Create)
	}

	usersC.Templates.SignIn = views.Must(views.ParseFS(
		templates.FS, "signin.gohtml", "tailwind.gohtml"))

	usersC.Templates.LoggedIn = views.Must(views.ParseFS(
		templates.FS, "home.gohtml", "tailwind.gohtml"))

	r.Get("/signin", usersC.SignIn)
	r.Post("/signin", usersC.ProcessSignIn)

	usersC.Templates.Home = views.Must(views.ParseFS(
		templates.FS, "home.gohtml", "tailwind.gohtml"))

	usersC.Templates.Profile = views.Must(views.ParseFS(
		templates.FS, "profile.gohtml", "tailwind.gohtml"))

	usersC.Templates.AdminPosts = views.Must(views.ParseFS(
		templates.FS, "admin-posts.gohtml", "tailwind.gohtml"))

	usersC.Templates.UserPosts = views.Must(views.ParseFS(
		templates.FS, "user-posts.gohtml", "tailwind.gohtml"))

	usersC.Templates.APIAccess = views.Must(views.ParseFS(
		templates.FS, "api-access.gohtml", "tailwind.gohtml"))

	usersC.Templates.PostEditor = views.Must(views.ParseFS(
		templates.FS, "post-editor.gohtml", "tailwind.gohtml"))

	categoriesC.Templates.Manage = views.Must(views.ParseFS(
		templates.FS, "admin-categories.gohtml", "tailwind.gohtml"))

	// Initialize Slides templates
	slidesC.Templates.AdminSlides = views.Must(views.ParseFS(
		templates.FS, "admin-slides.gohtml", "tailwind.gohtml"))
	
	slidesC.Templates.SlideEditor = views.Must(views.ParseFS(
		templates.FS, "slide-editor.gohtml", "tailwind.gohtml"))
	
	slidesC.Templates.SlidesList = views.Must(views.ParseFS(
		templates.FS, "slides-list.gohtml", "tailwind.gohtml"))
	
	slidesC.Templates.SlidePresentation = views.Must(views.ParseFS(
		templates.FS, "slide-presentation.gohtml", "tailwind.gohtml"))

	// Initialize Analytics controller
	analyticsC := controllers.Analytics{
		AnalyticsService: analyticsService,
		SessionService:   &sessionService,
	}

	// Initialize System templates
	systemC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-system.gohtml", "tailwind.gohtml"))

	// Initialize Analytics templates
	analyticsC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-analytics.gohtml", "tailwind.gohtml"))

	r.Get("/", usersC.Home)
	r.Get("/admin/posts", usersC.AdminPosts)
	r.Get("/admin/posts/new", usersC.NewPost)
	r.Post("/admin/posts", usersC.CreatePost)
	r.Post("/admin/posts/from-file", usersC.CreatePostFromFile)
	r.Get("/admin/posts/{postID}/edit", usersC.EditPost)
	r.Post("/admin/posts/{postID}", usersC.UpdatePost)
	r.Delete("/api/admin/posts", usersC.DeletePosts)
	r.Post("/admin/uploads", usersC.UploadImage)
	r.Post("/admin/uploads/multiple", usersC.UploadMultipleImages)
	r.Get("/admin/uploads/list", usersC.ListUploadedImages)
	r.Delete("/admin/uploads", usersC.DeleteImage)
	r.Post("/admin/preview", usersC.PreviewRender)

	// Image Metadata Routes
	r.Get("/api/admin/images", usersC.ListTrackedImages)
	r.Put("/api/admin/image-metadata", usersC.SaveImageMetadata)
	r.Get("/api/admin/image-metadata", usersC.GetImageMetadata)
	r.Delete("/api/admin/image-metadata", usersC.DeleteImageMetadata)
	r.Post("/api/admin/image-metadata/bulk", usersC.GetImageMetadataBulk)

	r.Get("/my-posts", usersC.UserPosts)
	r.Get("/api-access", usersC.APIAccess)

	// Category Management Routes
	r.Get("/admin/categories", categoriesC.Manage)
	r.Post("/admin/categories", categoriesC.CreateCategoryForm)
	r.Post("/admin/categories/{id}", categoriesC.UpdateCategoryForm)
	r.Post("/admin/categories/{id}/delete", categoriesC.DeleteCategoryForm)

	// Slides Routes
	r.Get("/slides", slidesC.PublicSlidesList)
	r.Get("/slides/{slug}", slidesC.ViewSlide)
	
	// Admin Slides Routes
	r.Get("/admin/slides", slidesC.AdminSlides)
	r.Get("/admin/slides/new", slidesC.NewSlide)
	r.Post("/admin/slides", slidesC.CreateSlide)
	r.Get("/admin/slides/{slideID}/edit", slidesC.EditSlide)
	r.Post("/admin/slides/{slideID}", slidesC.UpdateSlide)
	r.Post("/admin/slides/{slideID}/delete", slidesC.DeleteSlide)
	r.Post("/admin/slides/preview", slidesC.PreviewSlide)

	// System Information Routes
	r.Get("/admin/system", systemC.Dashboard)
	r.Get("/api/admin/system", systemC.GetSystemInfoJSON)
	r.Get("/api/admin/db/export", systemC.ExportDatabase)
	r.Post("/api/admin/db/import", systemC.ImportDatabase)

	// External Systems Routes
	r.Get("/api/admin/external-systems", systemC.ListExternalSystems)
	r.Get("/api/admin/external-systems/{id}", systemC.GetExternalSystem)
	r.Post("/api/admin/external-systems", systemC.CreateExternalSystem)
	r.Put("/api/admin/external-systems/{id}", systemC.UpdateExternalSystem)
	r.Delete("/api/admin/external-systems/{id}", systemC.DeleteExternalSystem)
	r.Post("/api/admin/external-systems/{id}/test", systemC.TestExternalConnection)
	r.Post("/api/admin/external-systems/{id}/sync/preview", systemC.PreviewSync)
	r.Post("/api/admin/external-systems/{id}/sync/execute", systemC.ExecuteSync)
	r.Get("/api/admin/external-systems/{id}/sync/logs", systemC.GetSyncLogs)

	// Analytics Routes
	r.Get("/admin/analytics", analyticsC.Dashboard)
	r.Get("/api/admin/analytics", analyticsC.GetAnalyticsJSON)
	r.Get("/api/admin/analytics/visitor", analyticsC.GetVisitorDetail)

	// Cloudinary Settings Routes
	r.Get("/api/admin/cloudinary", systemC.GetCloudinarySettings)
	r.Post("/api/admin/cloudinary", systemC.SaveCloudinarySettings)
	r.Delete("/api/admin/cloudinary", systemC.DeleteCloudinarySettings)
	r.Post("/api/admin/cloudinary/test", systemC.TestCloudinaryConnection)
	r.Post("/api/admin/cloudinary/signature", systemC.GetCloudinarySignature)
	r.Get("/api/admin/upload-config", usersC.GetUploadConfig)

	r.Get("/users/me", usersC.CurrentUser)
	r.Post("/users/password", usersC.UpdatePassword)
	r.Post("/users/email", usersC.UpdateEmail)
	r.Post("/users/api-tokens", usersC.CreateAPIToken)
	r.Post("/users/api-tokens/revoke", usersC.RevokeAPIToken)
	r.Post("/users/api-tokens/delete", usersC.DeleteAPIToken)

	// JSON API endpoints for AJAX operations
	r.Post("/api/users/api-tokens", usersC.CreateAPITokenJSON)
	r.Post("/api/users/api-tokens/revoke", usersC.RevokeAPITokenJSON)
	r.Delete("/api/users/api-tokens/{token_id}", usersC.DeleteAPITokenJSON)
	r.Get("/api/users/api-tokens", usersC.GetAPITokensJSON)
	r.Get("/users/logout", usersC.Logout)

	// Logout redirect route for convenience
	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/users/logout", http.StatusFound)
	})

	blogC.Templates.Post = views.Must(views.ParseFS(
		templates.FS, "blogpost.gohtml", "tailwind.gohtml"))

	// Define a route for the blog post
	r.Get("/blog/{slug}", blogC.GetBlogPost)

	// Public API for lazy loading posts
	r.Get("/api/posts/load-more", usersC.LoadMorePosts)

	// Public search API
	r.Get("/api/search", searchC.HandleSearch)

	// RSS Feed
	r.Get("/rss", rssHandler(&postService))

	// REST API endpoints for users
	r.Route("/api/users", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", usersC.ListUsers)
		r.Post("/", usersC.CreateUser)
	})

	r.Route("/api/posts", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", getAllPosts)
		r.Get("/formatted", getFormattedPosts)
		r.Get("/{postID}", getPostByID)
		r.Post("/", createPost)
		r.Post("/from-file", usersC.CreatePostFromFile)
	})

	r.Route("/api/categories", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", categoriesC.ListCategories)
		r.Post("/", categoriesC.CreateCategory)
		r.Get("/{id}", categoriesC.GetCategory)
		r.Put("/{id}", categoriesC.UpdateCategory)
		r.Delete("/{id}", categoriesC.DeleteCategory)
	})

	// go-draw canvas editor — use /data/draw-data for persistent storage
	drawStore, err := godrawstore.NewFileStore("/data/draw-data")
	if err != nil {
		logger.Fatal().Err(err).Msg("could not initialize go-draw store")
	}
	drawHandler, err := godraw.New(godraw.WithBasePath("/draw"), godraw.WithStore(drawStore))
	if err != nil {
		logger.Fatal().Err(err).Msg("could not initialize go-draw")
	}
	r.Handle("/draw/*", drawHandler.Handler())

	// Define a custom 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		// You can render a custom 404 page here
		// For simplicity, let's just return a plain text response
		http.ServeFile(w, r, "templates/NotFoundPage.gohtml")
	})

	logger.Info().Str("addr", *listenAddr).Msg("server listening")

	// Serve favicon at root level for both GET and HEAD requests
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		http.ServeFile(w, r, "./static/favicon.svg")
	})

	// Serve static files with cache headers
	staticFileServer := http.FileServer(http.Dir("./static/"))
	r.Handle("/static/*", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".js"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasSuffix(path, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Header().Set("Cache-Control", "public, max-age=86400")
		case strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".jpg"),
			strings.HasSuffix(path, ".jpeg"), strings.HasSuffix(path, ".webp"),
			strings.HasSuffix(path, ".gif"), strings.HasSuffix(path, ".ico"):
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}
		staticFileServer.ServeHTTP(w, r)
	})))

	// Keep legacy CSS route for backward compatibility
	cssFileServer := http.FileServer(http.Dir("./css/"))
	r.Handle("/css/*", http.StripPrefix("/css/", cssFileServer))

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	server.ListenAndServe()
}

func getAllPosts(w http.ResponseWriter, r *http.Request) {

	postService := models.PostService{
		DB: DB,
	}

	posts, err := postService.GetTopPosts()
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	// Send the posts as JSON response
	jsonResponse(w, posts, http.StatusOK)
}

// FormattedPost represents a post in the requested API format
type FormattedPost struct {
	Date       string   `json:"date"`
	Title      string   `json:"title"`
	Categories []string `json:"categories"`
	ReadTime   string   `json:"read_time"`
	Link       string   `json:"link"`
}

func getFormattedPosts(w http.ResponseWriter, r *http.Request) {
	postService := models.PostService{
		DB: DB,
	}

	// Get the 5 latest posts with user information
	posts, err := postService.GetTopPosts()
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	// Format posts according to the requested structure
	var formattedPosts []FormattedPost
	
	// Get the request host to construct full URLs
	host := r.Host
	if host == "" {
		host = "localhost:8080" // fallback
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	for _, post := range posts.Posts {
		if !post.IsPublished {
			continue // Skip unpublished posts
		}

		// Format date to "January 2, 2006" format
		formattedDate := post.PublicationDate
		if formattedDate == "" {
			formattedDate = post.CreatedAt
		}

		// Calculate reading time (approximately 200 words per minute)
		wordCount := len(strings.Fields(post.Content))
		readingMinutes := (wordCount + 199) / 200 // Round up
		if readingMinutes < 1 {
			readingMinutes = 1
		}
		readTime := fmt.Sprintf("%d min read", readingMinutes)

		// Get categories (placeholder for now since we need to implement category fetching)
		var categories []string
		for _, cat := range post.Categories {
			categories = append(categories, cat.Name)
		}
		if len(categories) == 0 {
			categories = []string{"General"} // default category
		}

		// Construct full link
		link := fmt.Sprintf("%s://%s/blog/%s", scheme, host, post.Slug)

		formattedPost := FormattedPost{
			Date:       formattedDate,
			Title:      post.Title,
			Categories: categories,
			ReadTime:   readTime,
			Link:       link,
		}

		formattedPosts = append(formattedPosts, formattedPost)
	}

	// Send the formatted posts as JSON response
	jsonResponse(w, formattedPosts, http.StatusOK)
}

func getPostByID(w http.ResponseWriter, r *http.Request) {
	postIDStr := chi.URLParam(r, "postID")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	postService := models.PostService{DB: DB}
	post, err := postService.GetByID(postID)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, post, http.StatusOK)
}

func createPost(w http.ResponseWriter, r *http.Request) {

	postService := models.PostService{
		DB: DB,
	}

	newPost := models.Post{}
	// Decode the JSON request to newPost
	err := json.NewDecoder(r.Body).Decode(&newPost)
	if err != nil {
		logger.Error().Err(err).Msg("error decoding JSON")
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Create a new post using the postService
	post, err := postService.Create(newPost.UserID, newPost.CategoryID, newPost.Title, newPost.Content, newPost.IsPublished, newPost.Featured, newPost.FeaturedImageURL, newPost.Slug)
	if err != nil {
		logger.Error().Err(err).Msg("error creating post")
		http.Error(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, post, http.StatusCreated)
}

// jsonResponse sends a JSON response with the given data and status code.
func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// rssHandler returns an RSS 2.0 feed of published posts.
func rssHandler(ps *models.PostService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		posts, err := ps.GetTopPostsWithPagination(20, 0)
		if err != nil || posts == nil {
			http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
			return
		}

		baseURL := os.Getenv("APP_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:22222"
		}

		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")

		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
  <title>Anshuman Biswas - Engineering Insights</title>
  <link>%s</link>
  <description>Deep dives into software architecture, cloud infrastructure, and scalable system design.</description>
  <language>en-us</language>
  <atom:link href="%s/rss" rel="self" type="application/rss+xml"/>
`, baseURL, baseURL)

		for _, post := range posts.Posts {
			if !post.IsPublished {
				continue
			}
			// Escape XML special characters in title and content
			title := xmlEscape(post.Title)
			link := fmt.Sprintf("%s/blog/%s", baseURL, post.Slug)
			desc := xmlEscape(utils.StripHTML(string(post.ContentHTML)))
			if len(desc) > 500 {
				desc = desc[:500] + "..."
			}
			fmt.Fprintf(w, `  <item>
    <title>%s</title>
    <link>%s</link>
    <guid>%s</guid>
    <description>%s</description>
    <pubDate>%s</pubDate>
  </item>
`, title, link, link, desc, post.CreatedAt)
		}

		fmt.Fprint(w, "</channel>\n</rss>")
	}
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

