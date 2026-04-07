// Path: main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"anshumanbiswas.com/blog/apm"
	"anshumanbiswas.com/blog/controllers"
	authmw "anshumanbiswas.com/blog/middleware"
	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/templates"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/version"
	"anshumanbiswas.com/blog/views"
	godraw "github.com/anchoo2kewl/go-draw"
	godrawstore "github.com/anchoo2kewl/go-draw/store"
	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	mimeSVG           = "image/svg+xml"
	headerCacheCtrl   = "Cache-Control"
	headerContentType = "Content-Type"

	routeAdminUploads       = "/admin/uploads"
	routeImageMetadata      = "/api/admin/image-metadata"
	routeExternalSystemByID = "/api/admin/external-systems/{id}"
	routeCloudinary         = "/api/admin/cloudinary"
	routeBrevo              = "/api/admin/brevo"
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

	// Initialise APM (provider-agnostic via OpenTelemetry).
	// Set APM_ENABLED=true to activate. Switch providers by changing
	// OTEL_EXPORTER_OTLP_ENDPOINT — no code changes required.
	apmCfg := apm.ConfigFromEnv(version.Version)
	apmShutdown, err := apm.Init(context.Background(), apmCfg)
	if err != nil {
		logger.Warn().Err(err).Msg("apm: failed to initialise, continuing without tracing")
		apmShutdown = func(context.Context) error { return nil }
	} else if apmCfg.Enabled {
		logger.Info().Str("endpoint", apmCfg.Endpoint).Msg("apm: tracing enabled")
	}
	defer func() {
		if err := apmShutdown(context.Background()); err != nil {
			logger.Warn().Err(err).Msg("apm: shutdown error")
		}
	}()

	// Continuous profiling — sends CPU, heap, goroutine and mutex profiles to
	// the Datadog agent. Activate with DD_PROFILING_ENABLED=true.
	profilerStop, err := apm.StartProfiling(apmCfg)
	if err != nil {
		logger.Warn().Err(err).Msg("apm: profiling failed to start, continuing without profiling")
	} else if os.Getenv("DD_PROFILING_ENABLED") == "true" {
		logger.Info().Msg("apm: continuous profiling enabled")
	}
	defer profilerStop()

	apiToken := os.Getenv("API_TOKEN")

	if apiToken == "" {
		logger.Fatal().Msg("API token not set in environment variable: API_TOKEN")
	} else {
		logger.Info().Msg("API token loaded")
	}

	githubClientID := os.Getenv("GH_CLIENT_ID")
	githubClientSecret := os.Getenv("GH_CLIENT_SECRET")
	oauthStateSecret := os.Getenv("OAUTH_STATE_SECRET")
	appURL := os.Getenv("APP_URL")

	listenAddr := flag.String("listen-addr", ":"+getAppPort(), "server listen address")
	flag.Parse()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Compress(5, "text/html", "text/css", "application/javascript", "application/json", mimeSVG))
	r.Use(authmw.TracingMiddleware())
	// Performance headers: tell clients/CDNs that encoding varies and opt-in to DNS prefetch
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-DNS-Prefetch-Control", "on")
			next.ServeHTTP(w, r)
		})
	})

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

	// Reader engagement tracker
	engagementService := &models.EngagementService{DB: DB}
	engagementController := controllers.EngagementController{Service: engagementService}

	userService := models.UserService{
		DB: DB,
	}

	userActivityService := models.UserActivityService{
		DB: DB,
	}

	sessionService := models.SessionService{
		DB: DB,
	}

	apiTokenService := models.APITokenService{
		DB: DB,
	}

	// Initialize IP ban cache and rules service
	ipBanCache := models.NewIPBanCache()
	ipRulesService := models.NewIPRulesService(DB, ipBanCache)
	defer ipRulesService.Shutdown()

	// Pre-populate cache from DB before middleware is registered
	if err := ipRulesService.LoadForCache(); err != nil {
		logger.Error().Err(err).Msg("failed to load IP rules into cache")
	}

	// Initialize crawler rules service
	crawlerRuleService, err := models.NewCrawlerRuleService(DB)
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize crawler rules service")
	}

	// BanMiddleware MUST be registered before TrackingMiddleware so banned IPs
	// are blocked before any page view is recorded.
	r.Use(authmw.BanMiddleware(ipBanCache))

	// Page view tracking middleware (records after response, zero latency)
	// Also enforces crawler blocking rules.
	r.Use(authmw.TrackingMiddleware(analyticsService, &sessionService, crawlerRuleService))

	// Initialize SystemService (before version endpoint which uses it)
	systemService := models.NewSystemService(DB, "migrations", startTime)

	// Health check endpoint (no auth required — for deployment verification)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := DB.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy"))
			return
		}
		w.Write([]byte("ok"))
	})

	// robots.txt — crawler directives
	r.Get("/robots.txt", robotsTxtHandler())

	// IndexNow key file — search engines fetch this to verify domain ownership
	r.Get("/"+controllers.IndexNowKey+".txt", controllers.IndexNowKeyHandler)

	// Reader engagement beacon (public, no auth) + admin read endpoint
	r.Post("/api/engagement", engagementController.Ingest)

	// Version endpoint — token-protected, rich response matching pingrly format
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		tok := os.Getenv("VERSION_TOKEN")
		if tok == "" {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "version endpoint disabled"})
			return
		}
		if r.Header.Get("X-Version-Token") != tok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid or missing version token"})
			return
		}

		hostname, _ := os.Hostname()
		env := os.Getenv("APP_ENV")
		if env == "" {
			env = "development"
		}

		// Backend info
		backend := map[string]string{
			"version":    version.Version,
			"git_commit": version.GitCommit,
			"build_time": version.BuildTime,
			"go_version": version.GoVersion,
			"platform":   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		}

		// Runtime info
		runtimeInfo := map[string]interface{}{
			"hostname":       hostname,
			"port":           getAppPort(),
			"environment":    env,
			"pid":            os.Getpid(),
			"uptime_seconds": int64(time.Since(startTime).Seconds()),
			"started_at":     startTime.UTC().Format(time.RFC3339),
		}

		// Database info
		dbInfo := map[string]interface{}{
			"type": "postgresql",
		}
		var pgVersion string
		if err := DB.QueryRow("SELECT version()").Scan(&pgVersion); err == nil {
			if parts := strings.SplitN(pgVersion, ",", 2); len(parts) > 0 {
				if fields := strings.Fields(parts[0]); len(fields) >= 2 {
					dbInfo["server_version"] = fields[0] + " " + fields[1]
				}
			}
		}
		migrationVersion, dirty := systemService.GetMigrationState()
		totalMigrations := systemService.CountMigrationFiles()
		dbInfo["current_version"] = migrationVersion
		dbInfo["total_migrations"] = totalMigrations
		dbInfo["up_to_date"] = migrationVersion >= totalMigrations
		dbInfo["dirty"] = dirty

		// Resource metrics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		resources := map[string]interface{}{
			"memory_alloc_mb":   float64(memStats.Alloc) / 1024 / 1024,
			"heap_inuse_mb":     float64(memStats.HeapInuse) / 1024 / 1024,
			"stack_inuse_mb":    float64(memStats.StackInuse) / 1024 / 1024,
			"goroutines":        runtime.NumGoroutine(),
			"num_gc":            memStats.NumGC,
			"gc_pause_total_ms": float64(memStats.PauseTotalNs) / 1e6,
			"gc_last_pause_ms":  float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6,
		}

		resp := map[string]interface{}{
			"backend":   backend,
			"runtime":   runtimeInfo,
			"database":  dbInfo,
			"resources": resources,
		}

		// Container metrics (cgroup)
		if cm := version.ReadContainerMetrics(); cm != nil {
			resp["container"] = cm
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

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

	postVersionService := &models.PostVersionService{DB: DB}

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
	slideService.MigrateFileContentToDB()

	// Initialize GuideService (early, needed by Users controller)
	guideService := models.GuideService{DB: DB}

	// Initialize BookService, BookGenreService, BookVersionService
	bookGenreService := models.BookGenreService{DB: DB}
	bookService := models.BookService{DB: DB}
	bookVersionService := &models.BookVersionService{DB: DB}

	// sitemap.xml — dynamic XML sitemap (depends on post/slide/guide/book/category services)
	r.Get("/sitemap.xml", sitemapHandler(&postService, &slideService, &guideService, &bookService, &categoryService, &bookGenreService))

	slideVersionService := &models.SlideVersionService{DB: DB}

	// Initialize SearchService
	searchService := &models.SearchService{
		DB: DB,
	}
	searchService.BackfillSlideContent()

	// Backfill avatar thumbnails for existing uploads
	go controllers.BackfillAvatarThumbnails()
	// Compress any oversized featured images (uploaded before auto-compression)
	go controllers.CompressOversizedImages()

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
		gowiki.WithUploadEndpoint(routeAdminUploads),
		gowiki.WithImageListEndpoint("/api/admin/images"),
		gowiki.WithImageMetadataEndpoint(routeImageMetadata),
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
		DB:                   DB,
		UserService:          &userService,
		SessionService:       &sessionService,
		PostService:          &postService,
		PostVersionService:   postVersionService,
		APITokenService:      &apiTokenService,
		CategoryService:      &categoryService,
		CloudinaryService:    &cloudinaryService,
		ImageMetadataService: &imageMetadataService,
		UserActivityService:  &userActivityService,
		SlideService:         &slideService,
		GuideService:         &guideService,
		BlogWiki:             blogWiki,
	}

	// Initialize BrevoService
	brevoService := models.BrevoService{
		DB: DB,
	}

	// Initialize AdminUsers controller
	adminUsersC := controllers.AdminUsers{
		UserActivityService: &userActivityService,
		SessionService:      &sessionService,
		UserService:         &userService,
		BrevoService:        &brevoService,
	}

	// Initialize Blog controller
	blogC := controllers.Blog{
		DB:                 DB,
		BlogService:        blogService,
		SessionService:     &sessionService,
		PostVersionService: postVersionService,
	}

	// Initialize PostVersions controller
	postVersionsC := controllers.PostVersions{
		PostVersionService: postVersionService,
		SessionService:     &sessionService,
		PostService:        &postService,
	}

	// Initialize Categories controller
	categoriesC := controllers.Categories{
		CategoryService: &categoryService,
		PostService:     &postService,
		SlideService:    &slideService,
		GuideService:    &guideService,
		SessionService:  &sessionService,
	}

	// Initialize Slides controller
	slidesC := controllers.Slides{
		SlideService:        &slideService,
		SlideVersionService: slideVersionService,
		SessionService:      &sessionService,
		CategoryService:     &categoryService,
	}

	// Initialize Guides controller
	guidesC := controllers.Guides{
		GuideService:    &guideService,
		SessionService:  &sessionService,
		CategoryService: &categoryService,
		BlogWiki:        blogWiki,
	}

	// Initialize Books controller
	booksC := controllers.Books{
		BookService:        &bookService,
		BookVersionService: bookVersionService,
		SessionService:     &sessionService,
		BookGenreService:   &bookGenreService,
		BlogWiki:           blogWiki,
	}

	// Initialize SlideVersions controller
	slideVersionsC := controllers.SlideVersions{
		SlideVersionService: slideVersionService,
		SessionService:      &sessionService,
		SlideService:        &slideService,
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
	siteSettingsService := models.NewSiteSettingsService(DB)
	// Wire site settings into templates so {{siteConfig "key" "fallback"}} works.
	views.SiteConfigFunc = siteSettingsService.Get

	systemC := controllers.System{
		SystemService:         systemService,
		DatabaseBackupService: databaseBackupService,
		SessionService:        &sessionService,
		ExternalSystemService: &externalSystemService,
		SyncClient:            &syncClient,
		CloudinaryService:     &cloudinaryService,
		BrevoService:          &brevoService,
		SiteSettingsService:   siteSettingsService,
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

	usersC.Templates.UserProfile = views.Must(views.ParseFS(
		templates.FS, "user-profile.gohtml", "tailwind.gohtml"))

	categoriesC.Templates.Manage = views.Must(views.ParseFS(
		templates.FS, "admin-categories.gohtml", "tailwind.gohtml"))
	categoriesC.Templates.TagPage = views.Must(views.ParseFS(
		templates.FS, "tag-page.gohtml", "tailwind.gohtml"))

	// Initialize Slides templates
	slidesC.Templates.AdminSlides = views.Must(views.ParseFS(
		templates.FS, "admin-slides.gohtml", "tailwind.gohtml"))

	slidesC.Templates.SlideEditor = views.Must(views.ParseFS(
		templates.FS, "slide-editor.gohtml", "tailwind.gohtml"))

	slidesC.Templates.SlidesList = views.Must(views.ParseFS(
		templates.FS, "slides-list.gohtml", "tailwind.gohtml"))

	slidesC.Templates.SlidePresentation = views.Must(views.ParseFS(
		templates.FS, "slide-presentation.gohtml", "tailwind.gohtml"))

	slidesC.Templates.SlidePassword = views.Must(views.ParseFS(
		templates.FS, "slide-password.gohtml", "tailwind.gohtml"))

	// Initialize Guides templates
	guidesC.Templates.GuidesList = views.Must(views.ParseFS(
		templates.FS, "guides-list.gohtml", "tailwind.gohtml"))
	guidesC.Templates.GuidePage = views.Must(views.ParseFS(
		templates.FS, "guide-page.gohtml", "tailwind.gohtml"))
	guidesC.Templates.AdminGuides = views.Must(views.ParseFS(
		templates.FS, "admin-guides.gohtml", "tailwind.gohtml"))
	guidesC.Templates.GuideEditor = views.Must(views.ParseFS(
		templates.FS, "guide-editor.gohtml", "tailwind.gohtml"))

	// Initialize Books templates
	booksC.Templates.BooksList = views.Must(views.ParseFS(
		templates.FS, "books-list.gohtml", "tailwind.gohtml"))
	booksC.Templates.BookPage = views.Must(views.ParseFS(
		templates.FS, "book-page.gohtml", "tailwind.gohtml"))
	booksC.Templates.AdminBooks = views.Must(views.ParseFS(
		templates.FS, "admin-books.gohtml", "tailwind.gohtml"))
	booksC.Templates.BookEditor = views.Must(views.ParseFS(
		templates.FS, "book-editor.gohtml", "tailwind.gohtml"))

	// Initialize Analytics controller
	analyticsC := controllers.Analytics{
		DB:               DB,
		AnalyticsService: analyticsService,
		SessionService:   &sessionService,
	}

	// Initialize System templates
	systemC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-system.gohtml", "tailwind.gohtml"))

	// Initialize Analytics templates
	analyticsC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-analytics.gohtml", "tailwind.gohtml"))

	// Initialize Security controller
	securityC := controllers.Security{
		IPRulesService: ipRulesService,
		SessionService: &sessionService,
	}
	securityC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-security.gohtml", "tailwind.gohtml"))

	// Initialize AdminUsers templates
	adminUsersC.Templates.Dashboard = views.Must(views.ParseFS(
		templates.FS, "admin-users.gohtml", "tailwind.gohtml"))

	r.Get("/", usersC.Home)
	r.Get("/admin/posts", usersC.AdminPosts)
	r.Get("/admin/posts/new", usersC.NewPost)
	r.Post("/admin/posts", usersC.CreatePost)
	r.Post("/admin/posts/from-file", usersC.CreatePostFromFile)
	r.Get("/admin/posts/{postID}/edit", usersC.EditPost)
	r.Post("/admin/posts/{postID}", usersC.UpdatePost)
	r.Delete("/api/admin/posts", usersC.DeletePosts)
	r.Post(routeAdminUploads, usersC.UploadImage)
	r.Post(routeAdminUploads+"/multiple", usersC.UploadMultipleImages)
	r.Get(routeAdminUploads+"/list", usersC.ListUploadedImages)
	r.Delete(routeAdminUploads, usersC.DeleteImage)
	r.Post("/admin/preview", usersC.PreviewRender)

	// Image Metadata Routes
	r.Get("/api/admin/images", usersC.ListTrackedImages)
	r.Put(routeImageMetadata, usersC.SaveImageMetadata)
	r.Get(routeImageMetadata, usersC.GetImageMetadata)
	r.Delete(routeImageMetadata, usersC.DeleteImageMetadata)
	r.Post(routeImageMetadata+"/bulk", usersC.GetImageMetadataBulk)

	r.Get("/my-posts", usersC.UserPosts)
	r.Get("/api-access", usersC.APIAccess)

	// Category Management Routes
	r.Get("/admin/categories", categoriesC.Manage)
	r.Post("/admin/categories", categoriesC.CreateCategoryForm)
	r.Post("/admin/categories/{id}", categoriesC.UpdateCategoryForm)
	r.Post("/admin/categories/{id}/delete", categoriesC.DeleteCategoryForm)

	// Slides Routes
	r.Get("/tags/{name}", categoriesC.TagPage)
	r.Get("/slides", slidesC.PublicSlidesList)
	r.Get("/slides/{slug}", slidesC.ViewSlide)
	r.Post("/slides/{slug}/verify", slidesC.VerifySlidePassword)

	// Admin Slides Routes
	r.Get("/admin/slides", slidesC.AdminSlides)
	r.Get("/admin/slides/new", slidesC.NewSlide)
	r.Post("/admin/slides", slidesC.CreateSlide)
	r.Get("/admin/slides/{slideID}/edit", slidesC.EditSlide)
	r.Post("/admin/slides/{slideID}", slidesC.UpdateSlide)
	r.Post("/admin/slides/{slideID}/delete", slidesC.DeleteSlide)
	r.Post("/admin/slides/preview", slidesC.PreviewSlide)
	r.Post("/admin/slides/upload-image", slidesC.UploadSlideImage)

	// Public Guide Routes
	r.Get("/guides", guidesC.PublicGuidesList)
	r.Get("/guides/{slug}", guidesC.ViewGuide)

	// Admin Guide Routes
	r.Get("/admin/guides", guidesC.AdminGuides)
	r.Get("/admin/guides/new", guidesC.NewGuide)
	r.Post("/admin/guides", guidesC.CreateGuide)
	r.Get("/admin/guides/{guideID}/edit", guidesC.EditGuide)
	r.Post("/admin/guides/{guideID}", guidesC.UpdateGuide)
	r.Post("/admin/guides/{guideID}/delete", guidesC.DeleteGuide)
	r.Post("/admin/guides/preview", guidesC.PreviewGuide)

	// Public Book Routes
	r.Get("/books", booksC.PublicBooksList)
	r.Get("/books/{slug}", booksC.ViewBook)
	r.Get("/books/{slug}/buy", booksC.BuyBook)
	r.Get("/books/genre/{name}", booksC.GenrePage)
	r.Get("/books/author/{name}", booksC.AuthorPage)
	r.Get("/books/publisher/{name}", booksC.PublisherPage)

	// Admin Book Routes
	r.Get("/admin/books", booksC.AdminBooks)
	r.Get("/admin/books/new", booksC.NewBook)
	r.Post("/admin/books", booksC.CreateBook)
	r.Get("/admin/books/{bookID}/edit", booksC.EditBook)
	r.Post("/admin/books/{bookID}", booksC.UpdateBook)
	r.Post("/admin/books/{bookID}/delete", booksC.DeleteBook)
	r.Post("/admin/books/preview", booksC.PreviewBook)

	// Slide Version API Routes
	r.Get("/api/slides/{slideID}/versions", slideVersionsC.HandleListVersions)
	r.Get("/api/slides/{slideID}/versions/{versionNum}", slideVersionsC.HandleGetVersion)
	r.Post("/api/slides/{slideID}/versions/{versionNum}/restore", slideVersionsC.HandleRestoreVersion)
	r.Delete("/api/slides/{slideID}/versions/{versionNum}", slideVersionsC.HandleDeleteVersion)

	// Slide Autosave & Import API
	r.Post("/api/admin/slides/{slideID}/autosave", slidesC.AutoSave)
	r.Post("/api/admin/slides/import-pptx", slidesC.ImportPPTX)
	r.Post("/api/admin/slides/{slideID}/reimport-pptx", slidesC.ReimportPPTX)

	// System Information Routes
	r.Get("/admin/system", systemC.Dashboard)
	r.Get("/api/admin/system", systemC.GetSystemInfoJSON)
	r.Get("/api/admin/db/export", systemC.ExportDatabase)
	r.Post("/api/admin/db/import", systemC.ImportDatabase)

	// External Systems Routes
	r.Get("/api/admin/external-systems", systemC.ListExternalSystems)
	r.Get(routeExternalSystemByID, systemC.GetExternalSystem)
	r.Post("/api/admin/external-systems", systemC.CreateExternalSystem)
	r.Put(routeExternalSystemByID, systemC.UpdateExternalSystem)
	r.Delete(routeExternalSystemByID, systemC.DeleteExternalSystem)
	r.Post(routeExternalSystemByID+"/test", systemC.TestExternalConnection)
	r.Post(routeExternalSystemByID+"/sync/preview", systemC.PreviewSync)
	r.Post(routeExternalSystemByID+"/sync/execute", systemC.ExecuteSync)
	r.Get(routeExternalSystemByID+"/sync/logs", systemC.GetSyncLogs)

	// Analytics Routes
	r.Get("/admin/analytics", analyticsC.Dashboard)
	r.Get("/api/admin/analytics", analyticsC.GetAnalyticsJSON)
	r.Get("/api/admin/analytics/visitor", analyticsC.GetVisitorDetail)

	// Crawler Analytics Route (admin)
	r.Get("/api/admin/analytics/crawlers", crawlerStatsHandler(analyticsService, &sessionService))

	// Crawler Rules Management Routes (admin)
	r.Get("/api/admin/crawlers/rules", crawlerRulesListHandler(crawlerRuleService, &sessionService))
	r.Post("/api/admin/crawlers/rules", crawlerRulesCreateHandler(crawlerRuleService, &sessionService))
	r.Put("/api/admin/crawlers/rules/{id}", crawlerRulesUpdateHandler(crawlerRuleService, &sessionService))
	r.Delete("/api/admin/crawlers/rules/{id}", crawlerRulesDeleteHandler(crawlerRuleService, &sessionService))

	// Engagement Management Routes (admin)
	r.Get("/api/admin/engagement", analyticsC.GetEngagementJSON)
	r.Get("/api/admin/engagement-summary", engagementController.GetSummary)
	r.Delete("/api/admin/engagement/comments/{id}", analyticsC.AdminDeleteComment)
	r.Delete("/api/admin/engagement/annotations/{id}", analyticsC.AdminDeleteAnnotation)

	// 404 Slug Tracking Routes (admin)
	r.Get("/api/admin/slug-404s", analyticsC.GetSlug404sJSON)
	r.Post("/api/admin/slug-404s/{id}/whitelist", analyticsC.WhitelistSlug404)
	r.Delete("/api/admin/slug-404s/{id}", analyticsC.DeleteSlug404)

	// Security Routes
	r.Get("/admin/security", securityC.Dashboard)
	r.Get("/api/admin/security/rules", securityC.ListRulesJSON)
	r.Post("/api/admin/security/ban", securityC.BanIP)
	r.Post("/api/admin/security/allow", securityC.AllowIP)
	r.Delete("/api/admin/security/rules", securityC.RemoveRule)

	// Admin User Management Routes
	r.Get("/admin/users", adminUsersC.Dashboard)
	r.Get("/api/admin/users", adminUsersC.GetUsersJSON)
	r.Post("/api/admin/users/create", adminUsersC.CreateUser)
	r.Get("/api/admin/users/{id}/activity", adminUsersC.GetUserActivityJSON)
	r.Post("/api/admin/users/{id}/role", adminUsersC.UpdateUserRole)
	r.Post("/api/admin/users/{id}/password", adminUsersC.AdminResetPassword)
	r.Put("/api/admin/users/{id}", adminUsersC.AdminUpdateUser)

	// Cloudinary Settings Routes
	r.Get(routeCloudinary, systemC.GetCloudinarySettings)
	r.Post(routeCloudinary, systemC.SaveCloudinarySettings)
	r.Delete(routeCloudinary, systemC.DeleteCloudinarySettings)
	r.Post(routeCloudinary+"/test", systemC.TestCloudinaryConnection)
	r.Post(routeCloudinary+"/signature", systemC.GetCloudinarySignature)

	// Brevo Email Settings Routes
	r.Get(routeBrevo, systemC.GetBrevoSettings)
	r.Post(routeBrevo, systemC.SaveBrevoSettings)
	r.Delete(routeBrevo, systemC.DeleteBrevoSettings)
	r.Post(routeBrevo+"/test", systemC.TestBrevoConnection)

	// Site settings (generic key-value)
	r.Get("/api/admin/site-settings/{key}", systemC.GetSiteSettings)
	r.Put("/api/admin/site-settings/{key}", systemC.SaveSiteSetting)
	// Cloudflare cache purge
	r.Post("/api/admin/cloudflare/purge", systemC.PurgeCloudflareCacheHandler)

	r.Get("/api/admin/upload-config", usersC.GetUploadConfig)

	r.Get("/users/me", usersC.CurrentUser)
	r.Get("/users/{username}", usersC.PublicProfile)
	r.Post("/users/password", usersC.UpdatePassword)
	r.Post("/users/email", usersC.UpdateEmail)
	r.Post("/users/name", usersC.UpdateName)
	r.Post("/users/bio", usersC.UpdateBio)
	r.Post("/users/avatar", usersC.UploadAvatar)
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

	// GitHub OAuth routes
	oauthC := controllers.OAuthController{
		DB:                 DB,
		UserService:        &userService,
		SessionService:     &sessionService,
		GitHubClientID:     githubClientID,
		GitHubClientSecret: githubClientSecret,
		AppURL:             appURL,
		StateSecret:        oauthStateSecret,
	}
	r.Get("/auth/github", oauthC.HandleGitHubLogin)
	r.Get("/auth/github/callback", oauthC.HandleGitHubCallback)

	// Comments routes
	commentsC := controllers.CommentsController{
		DB:           DB,
		BlogService:  blogService,
		GuideService: &guideService,
	}
	r.Get("/blog/{slug}/comments", commentsC.HandleListComments)
	r.Get("/guides/{slug}/comments", commentsC.HandleListGuideComments)
	r.Group(func(r chi.Router) {
		r.Use(authmw.AuthenticatedUser(&sessionService, &apiTokenService))
		r.Post("/blog/{slug}/comments", commentsC.HandleCreateComment)
		r.Post("/guides/{slug}/comments", commentsC.HandleCreateGuideComment)
		r.Delete("/comments/{commentID}", commentsC.HandleDeleteComment)
	})

	// Annotations routes
	annotationsC := controllers.AnnotationsController{DB: DB}
	r.Get("/blog/{slug}/annotations", annotationsC.HandleListAnnotations)
	r.Group(func(r chi.Router) {
		r.Use(authmw.AuthenticatedUser(&sessionService, &apiTokenService))
		r.Use(authmw.RequirePermission(func(p models.UserPermissions) bool { return p.CanComment }))
		r.Post("/blog/{slug}/annotations", annotationsC.HandleCreateAnnotation)
		r.Patch("/annotations/{annotationID}", annotationsC.HandleUpdateAnnotation)
		r.Delete("/annotations/{annotationID}", annotationsC.HandleDeleteAnnotation)
		r.Post("/annotations/{annotationID}/comments", annotationsC.HandleCreateAnnotationComment)
		r.Patch("/annotation-comments/{commentID}", annotationsC.HandleUpdateAnnotationComment)
		r.Delete("/annotation-comments/{commentID}", annotationsC.HandleDeleteAnnotationComment)
	})

	// Post version history API (editor+ only)
	r.Get("/api/posts/{postID}/versions", postVersionsC.HandleListVersions)
	r.Get("/api/posts/{postID}/versions/{versionNum}", postVersionsC.HandleGetVersion)
	r.Post("/api/posts/{postID}/versions/{versionNum}/restore", postVersionsC.HandleRestoreVersion)
	r.Delete("/api/posts/{postID}/versions/{versionNum}", postVersionsC.HandleDeleteVersion)

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
		r.Put("/{postID}", updatePost(&postService, postVersionService, &categoryService))
		r.Delete("/{postID}", deletePost(&postService))
	})

	r.Route("/api/categories", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", categoriesC.ListCategories)
		r.Post("/", categoriesC.CreateCategory)
		r.Get("/{id}", categoriesC.GetCategory)
		r.Put("/{id}", categoriesC.UpdateCategory)
		r.Delete("/{id}", categoriesC.DeleteCategory)
	})

	r.Route("/api/slides", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", listSlides(&slideService))
		r.Get("/{slideID}", getSlide(&slideService))
		r.Post("/", createSlide(&slideService))
		r.Put("/{slideID}", updateSlide(&slideService))
		r.Delete("/{slideID}", deleteSlide(&slideService))
	})

	// Guides API
	r.Route("/api/guides", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", listGuidesAPI(&guideService))
		r.Get("/{guideID}", getGuideAPI(&guideService))
		r.Post("/", createGuideAPI(&guideService))
		r.Put("/{guideID}", updateGuideAPI(&guideService))
		r.Delete("/{guideID}", deleteGuideAPI(&guideService))
	})

	// Books API
	r.Route("/api/books", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", listBooksAPI(&bookService))
		r.Get("/{bookID}", getBookAPI(&bookService))
		r.Post("/", createBookAPI(&bookService))
		r.Put("/{bookID}", updateBookAPI(&bookService))
		r.Delete("/{bookID}", deleteBookAPI(&bookService))
	})

	// Book Genres API
	r.Route("/api/book-genres", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/", listBookGenresAPI(&bookGenreService))
		r.Post("/", createBookGenreAPI(&bookGenreService))
		r.Put("/{genreID}", updateBookGenreAPI(&bookGenreService))
		r.Delete("/{genreID}", deleteBookGenreAPI(&bookGenreService))
	})

	// Comments API (token-authenticated)
	r.Route("/api/comments", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/post/{slug}", commentsC.HandleListComments)
		r.Post("/post/{slug}", commentsC.HandleCreateComment)
		r.Get("/guide/{slug}", commentsC.HandleListGuideComments)
		r.Post("/guide/{slug}", commentsC.HandleCreateGuideComment)
		r.Delete("/{commentID}", commentsC.HandleDeleteComment)
	})

	// Wiki API
	wikiPageService := &models.WikiPageService{DB: DB}
	wikiC := controllers.Wiki{WikiPageService: wikiPageService}
	r.Route("/api/wiki", func(r chi.Router) {
		r.Use(authmw.APIAuthMiddleware(apiToken, &apiTokenService))
		r.Get("/pages", wikiC.ListPages)
		r.Post("/pages", wikiC.CreatePage)
		r.Get("/pages/{pageID}", wikiC.GetPage)
		r.Put("/pages/{pageID}", wikiC.UpdatePage)
		r.Delete("/pages/{pageID}", wikiC.DeletePage)
		r.Get("/pages/{pageID}/content", wikiC.GetPageContent)
		r.Put("/pages/{pageID}/content", wikiC.UpdatePageContent)
		r.Get("/pages/{pageID}/versions", wikiC.ListVersions)
		r.Get("/pages/{pageID}/versions/{versionNum}", wikiC.GetVersion)
		r.Post("/pages/{pageID}/versions/{versionNum}/restore", wikiC.RestoreVersion)
		r.Delete("/pages/{pageID}/versions/{versionNum}", wikiC.DeleteVersionHandler)
		r.Get("/search", wikiC.SearchPages)
		r.Get("/autocomplete", wikiC.AutocompletePages)
	})

	// go-draw canvas editor — use /data/draw-data for persistent storage
	drawDataDir := "/data/draw-data"
	if dir := os.Getenv("DRAW_DATA_DIR"); dir != "" {
		drawDataDir = dir
	}
	drawStore, err := godrawstore.NewFileStore(drawDataDir)
	if err != nil {
		logger.Fatal().Err(err).Msg("could not initialize go-draw store")
	}
	drawHandler, err := godraw.New(godraw.WithBasePath("/draw"), godraw.WithStore(drawStore))
	if err != nil {
		logger.Fatal().Err(err).Msg("could not initialize go-draw")
	}

	// Wrap go-draw handler with auth: only admin/editor can create, edit,
	// save, delete, rename, or upload drawings. Everyone else gets read-only.
	drawAuth := drawAuthMiddleware(&sessionService, apiToken, &apiTokenService, drawHandler.Handler())
	r.Handle("/draw/*", drawAuth)

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
		w.Header().Set(headerContentType, mimeSVG)
		http.ServeFile(w, r, "./static/favicon.svg")
	})

	// Serve static files with cache headers
	staticFileServer := http.FileServer(http.Dir("./static/"))
	r.Handle("/static/*", http.StripPrefix("/static/", staticCacheMiddleware(staticFileServer)))

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

// staticCacheMiddleware sets appropriate Cache-Control headers based on file extension.
func staticCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".js"),
			strings.HasSuffix(path, ".woff2"), strings.HasSuffix(path, ".woff"):
			// Versioned assets: 1-year immutable cache — URL contains version query param
			w.Header().Set(headerCacheCtrl, "public, max-age=31536000, immutable")
		case strings.HasSuffix(path, ".svg"):
			w.Header().Set(headerContentType, mimeSVG)
			// SVGs are stable; cache 1 week with 30-day background revalidation
			w.Header().Set(headerCacheCtrl, "public, max-age=604800, stale-while-revalidate=2592000")
		case strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".jpg"),
			strings.HasSuffix(path, ".jpeg"), strings.HasSuffix(path, ".webp"),
			strings.HasSuffix(path, ".gif"), strings.HasSuffix(path, ".ico"):
			// Images: 7-day cache with 30-day background revalidation
			w.Header().Set(headerCacheCtrl, "public, max-age=604800, stale-while-revalidate=2592000")
		}
		next.ServeHTTP(w, r)
	})
}

// drawAuthMiddleware wraps the go-draw handler to enforce role-based access.
// Write operations (create, edit, save, delete, rename, upload) require
// admin or editor role. Read operations (view, data, static, list) are open
// to everyone. The list page injects CSS to hide the "+ New Drawing" button
// for users who are not admin/editor.
func drawAuthMiddleware(ss *models.SessionService, apiTokenStr string, apiTokenService *models.APITokenService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/draw")
		path = strings.TrimPrefix(path, "/")

		// Check if this is a write operation that requires editor/admin role.
		writeOp := false
		switch {
		case path == "new":
			writeOp = true
		case path == "api/new":
			writeOp = true
		case path == "api/upload":
			writeOp = true
		case strings.HasSuffix(path, "/edit"):
			writeOp = true
		case strings.HasSuffix(path, "/save"):
			writeOp = true
		case strings.HasSuffix(path, "/delete"):
			writeOp = true
		case strings.Contains(path, "api/") && strings.HasSuffix(path, "/rename"):
			writeOp = true
		case strings.Contains(path, "api/") && strings.HasSuffix(path, "/delete"):
			writeOp = true
		}

		if writeOp {
			// Try session auth first
			user, err := utils.IsUserLoggedIn(r, ss)
			authed := err == nil && user != nil && (models.IsAdmin(user.Role) || models.CanEditPosts(user.Role))

			// Fall back to API token auth (Bearer token in Authorization header)
			if !authed {
				if authHeader := r.Header.Get("Authorization"); authHeader != "" {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					if token == apiTokenStr {
						authed = true
					} else if apiTokenService != nil {
						if _, apiErr := apiTokenService.ValidateToken(token); apiErr == nil {
							authed = true
						}
					}
				}
			}

			if !authed {
				http.Error(w, "Unauthorized: sign in or API token required", http.StatusUnauthorized)
				return
			}
		}

		// Check if the current user can edit drawings.
		canEdit := false
		if user, err := utils.IsUserLoggedIn(r, ss); err == nil && user != nil {
			canEdit = models.IsAdmin(user.Role) || models.CanEditPosts(user.Role)
		}

		// All go-draw pages (list, viewer, editor) are internal tools — tell
		// search engines not to index them. Blog posts embed drawings via
		// iframes so the content is still reachable.
		isHTMLPage := !strings.HasPrefix(path, "static/") &&
			!strings.HasPrefix(path, "uploads/") &&
			!strings.HasPrefix(path, "api/") &&
			!strings.HasSuffix(path, "/data") &&
			!strings.HasSuffix(path, "/save")
		if isHTMLPage {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}

		// For the list page, hide "+", "Edit", "Delete" buttons for non-editors.
		if !canEdit && (path == "" || path == "/") {
			w = &drawHideButtonsWriter{
				ResponseWriter: w,
				css:            `.new-btn,.btn-edit,.btn-del{display:none!important}`,
			}
		}

		// For viewer pages (/draw/{id}), hide the "+" new-canvas button
		// rendered by canvas.js inside the iframe.
		if !canEdit && !writeOp && path != "" && path != "/" && isHTMLPage {
			w = &drawHideButtonsWriter{
				ResponseWriter: w,
				css:            `#btn-new-canvas{display:none!important}`,
			}
		}

		next.ServeHTTP(w, r)
	})
}

// drawHideButtonsWriter wraps http.ResponseWriter to inject a <style> tag
// that hides specific buttons in go-draw HTML responses.
type drawHideButtonsWriter struct {
	http.ResponseWriter
	css      string
	injected bool
}

func (rw *drawHideButtonsWriter) Write(data []byte) (int, error) {
	if !rw.injected {
		s := string(data)
		if idx := strings.Index(s, "</head>"); idx != -1 {
			s = s[:idx] + "<style>" + rw.css + "</style>" + s[idx:]
			rw.injected = true
			return rw.ResponseWriter.Write([]byte(s))
		}
	}
	return rw.ResponseWriter.Write(data)
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
	Date          string   `json:"date"`
	Title         string   `json:"title"`
	Categories    []string `json:"categories"`
	ReadTime      string   `json:"read_time"`
	Link          string   `json:"link"`
	Excerpt       string   `json:"excerpt,omitempty"`
	CoverImageURL string   `json:"cover_image_url,omitempty"`
}

func getFormattedPosts(w http.ResponseWriter, r *http.Request) {
	postService := models.PostService{
		DB: DB,
	}

	limit := 5
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > 50 {
				parsed = 50
			}
			limit = parsed
		}
	}

	// Get the latest posts with user information.
	posts, err := postService.GetTopPostsWithPagination(limit, 0)
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	// Format posts according to the requested structure
	var formattedPosts []FormattedPost

	// Get the request host to construct full URLs
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		host = "localhost:8080" // fallback
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" && r.TLS != nil {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
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
		link := normalizeFormattedPublicURL(fmt.Sprintf("%s://%s/blog/%s", scheme, host, post.Slug), scheme, host)
		coverImageURL := normalizeFormattedCoverURL(post.FeaturedImageURL, scheme, host)
		excerpt := formatPostExcerpt(post.Content, 40)

		formattedPost := FormattedPost{
			Date:          formattedDate,
			Title:         post.Title,
			Categories:    categories,
			ReadTime:      readTime,
			Link:          link,
			Excerpt:       excerpt,
			CoverImageURL: coverImageURL,
		}

		formattedPosts = append(formattedPosts, formattedPost)
	}

	// Send the formatted posts as JSON response
	jsonResponse(w, formattedPosts, http.StatusOK)
}

func normalizeFormattedPublicURL(raw, scheme, host string) string {
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "//") {
		return fmt.Sprintf("%s:%s", scheme, raw)
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
		return fmt.Sprintf("%s://%s", scheme, trimmed)
	}

	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, raw)
}

func normalizeFormattedCoverURL(raw, scheme, host string) string {
	if raw == "" || raw == "image.jpg" {
		return ""
	}

	if !strings.HasPrefix(raw, "/") {
		if strings.HasPrefix(raw, "static/") {
			raw = "/" + raw
		} else if strings.HasPrefix(raw, "uploads/") {
			raw = "/static/" + raw
		} else {
			raw = "/static/" + strings.TrimPrefix(raw, "/")
		}
	}

	if !strings.HasPrefix(raw, "/static/") {
		raw = "/static/" + strings.TrimPrefix(raw, "/")
	}

	return normalizeFormattedPublicURL(raw, scheme, host)
}

func formatPostExcerpt(content string, maxWords int) string {
	markers := []string{"<more-->", "<more -->", "&lt;more--&gt;", "&lt;more --&gt;"}
	for _, marker := range markers {
		if idx := strings.Index(content, marker); idx != -1 {
			content = content[:idx]
			break
		}
	}

	lines := strings.Split(content, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "![") ||
			strings.HasPrefix(trimmed, "---") ||
			strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "|") ||
			strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		trimmed = strings.NewReplacer(
			"**", "",
			"__", "",
			"`", "",
			"*", "",
			"_", "",
		).Replace(trimmed)
		trimmed = stripMarkdownLinksForAPI(trimmed)
		parts = append(parts, trimmed)
	}

	text := strings.Join(parts, " ")
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}

	return strings.Join(words[:maxWords], " ") + "..."
}

func stripMarkdownLinksForAPI(s string) string {
	for {
		start := strings.Index(s, "[")
		if start == -1 {
			return s
		}
		mid := strings.Index(s[start:], "](")
		if mid == -1 {
			return s
		}
		end := strings.Index(s[start+mid:], ")")
		if end == -1 {
			return s
		}
		linkText := s[start+1 : start+mid]
		s = s[:start] + linkText + s[start+mid+end+1:]
	}
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

func updatePost(ps *models.PostService, pvs *models.PostVersionService, cs *models.CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postIDStr := chi.URLParam(r, "postID")
		postID, err := strconv.Atoi(postIDStr)
		if err != nil {
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Title            string `json:"title"`
			Content          string `json:"content"`
			Slug             string `json:"slug"`
			IsPublished      *bool  `json:"is_published"`
			Featured         *bool  `json:"featured"`
			FeaturedImageURL string `json:"featured_image_url"`
			CategoryID       int    `json:"category_id"`
			Categories       []int  `json:"categories"`
			UserID           int    `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		// Load existing post to fill in defaults for unset fields
		existing, err := ps.GetByID(postID)
		if err != nil {
			http.Error(w, "Post not found", http.StatusNotFound)
			return
		}

		title := req.Title
		if title == "" {
			title = existing.Title
		}
		content := req.Content
		if content == "" {
			content = existing.Content
		}
		slug := req.Slug
		if slug == "" {
			slug = existing.Slug
		}
		isPublished := existing.IsPublished
		if req.IsPublished != nil {
			isPublished = *req.IsPublished
		}
		featured := existing.Featured
		if req.Featured != nil {
			featured = *req.Featured
		}
		featuredImageURL := req.FeaturedImageURL
		if featuredImageURL == "" {
			featuredImageURL = existing.FeaturedImageURL
		}
		categoryID := req.CategoryID
		if categoryID == 0 {
			categoryID = existing.CategoryID
		}

		if err := ps.Update(postID, categoryID, title, content, isPublished, featured, featuredImageURL, slug); err != nil {
			logger.Error().Err(err).Msg("error updating post")
			http.Error(w, "Failed to update post", http.StatusInternalServerError)
			return
		}

		// Create version snapshot
		userID := req.UserID
		if userID == 0 {
			userID = existing.UserID
		}
		_ = pvs.MaybeCreateVersion(postID, userID, title, content)

		// Update categories if provided
		if len(req.Categories) > 0 {
			_ = cs.AssignCategoriesToPost(postID, req.Categories)
		}

		updated, err := ps.GetByID(postID)
		if err != nil {
			http.Error(w, "Failed to fetch updated post", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, updated, http.StatusOK)
	}
}

func deletePost(ps *models.PostService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postIDStr := chi.URLParam(r, "postID")
		postID, err := strconv.Atoi(postIDStr)
		if err != nil {
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		if err := ps.Delete(postID); err != nil {
			logger.Error().Err(err).Msg("error deleting post")
			http.Error(w, "Failed to delete post", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func listSlides(ss *models.SlideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slides, err := ss.GetAllSlides()
		if err != nil {
			http.Error(w, "Failed to fetch slides", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, slides, http.StatusOK)
	}
}

func getSlide(ss *models.SlideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
		if err != nil {
			http.Error(w, "Invalid slide ID", http.StatusBadRequest)
			return
		}
		slide, err := ss.GetByID(slideID)
		if err != nil {
			http.Error(w, "Slide not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, slide, http.StatusOK)
	}
}

func createSlide(ss *models.SlideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID           int    `json:"user_id"`
			Title            string `json:"title"`
			Slug             string `json:"slug"`
			Content          string `json:"content"`
			IsPublished      bool   `json:"is_published"`
			Description      string `json:"description"`
			FeaturedImageURL string `json:"featured_image_url"`
			Metadata         string `json:"metadata"`
			Password         string `json:"password"`
			Categories       []int  `json:"categories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "Title is required", http.StatusBadRequest)
			return
		}
		if req.Metadata == "" {
			req.Metadata = "{}"
		}
		slide, err := ss.Create(req.UserID, req.Title, req.Slug, req.Content, req.IsPublished, req.Categories, req.Description, req.Metadata, req.Password, req.FeaturedImageURL)
		if err != nil {
			logger.Error().Err(err).Msg("error creating slide")
			http.Error(w, "Failed to create slide", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, slide, http.StatusCreated)
	}
}

func updateSlide(ss *models.SlideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
		if err != nil {
			http.Error(w, "Invalid slide ID", http.StatusBadRequest)
			return
		}
		var req struct {
			Title            string `json:"title"`
			Slug             string `json:"slug"`
			Content          string `json:"content"`
			IsPublished      *bool  `json:"is_published"`
			Description      string `json:"description"`
			FeaturedImageURL string `json:"featured_image_url"`
			Metadata         string `json:"metadata"`
			Password         string `json:"password"`
			Categories       []int  `json:"categories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		// Load existing to fill defaults
		existing, err := ss.GetByID(slideID)
		if err != nil {
			http.Error(w, "Slide not found", http.StatusNotFound)
			return
		}
		title := req.Title
		if title == "" {
			title = existing.Title
		}
		slug := req.Slug
		if slug == "" {
			slug = existing.Slug
		}
		content := req.Content
		if content == "" {
			content = string(existing.ContentHTML)
		}
		isPublished := existing.IsPublished
		if req.IsPublished != nil {
			isPublished = *req.IsPublished
		}
		description := req.Description
		if description == "" {
			description = existing.Description
		}
		featuredImageURL := req.FeaturedImageURL
		if featuredImageURL == "" {
			featuredImageURL = existing.FeaturedImageURL
		}

		if err := ss.Update(slideID, title, slug, content, isPublished, req.Categories, description, req.Metadata, req.Password, featuredImageURL); err != nil {
			logger.Error().Err(err).Msg("error updating slide")
			http.Error(w, "Failed to update slide", http.StatusInternalServerError)
			return
		}

		updated, err := ss.GetByID(slideID)
		if err != nil {
			http.Error(w, "Failed to fetch updated slide", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, updated, http.StatusOK)
	}
}

func deleteSlide(ss *models.SlideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
		if err != nil {
			http.Error(w, "Invalid slide ID", http.StatusBadRequest)
			return
		}
		if err := ss.Delete(slideID); err != nil {
			logger.Error().Err(err).Msg("error deleting slide")
			http.Error(w, "Failed to delete slide", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── Guide API handlers ──────────────────────────────────────────────

func listGuidesAPI(gs *models.GuideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guides, err := gs.GetAllGuides()
		if err != nil {
			http.Error(w, "Failed to fetch guides", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, guides, http.StatusOK)
	}
}

func getGuideAPI(gs *models.GuideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guideID, err := strconv.Atoi(chi.URLParam(r, "guideID"))
		if err != nil {
			http.Error(w, "Invalid guide ID", http.StatusBadRequest)
			return
		}
		guide, err := gs.GetByID(guideID)
		if err != nil {
			http.Error(w, "Guide not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, guide, http.StatusOK)
	}
}

func createGuideAPI(gs *models.GuideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID           int    `json:"user_id"`
			Title            string `json:"title"`
			Slug             string `json:"slug"`
			Content          string `json:"content"`
			Description      string `json:"description"`
			FeaturedImageURL string `json:"featured_image_url"`
			IsPublished      bool   `json:"is_published"`
			Categories       []int  `json:"categories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "Title is required", http.StatusBadRequest)
			return
		}
		if req.UserID == 0 {
			req.UserID = 1 // default to first user
		}
		guide, err := gs.Create(req.UserID, req.Title, req.Slug, req.Content, req.Description, req.FeaturedImageURL, req.IsPublished, req.Categories)
		if err != nil {
			logger.Error().Err(err).Msg("error creating guide")
			http.Error(w, "Failed to create guide: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, guide, http.StatusCreated)
	}
}

func updateGuideAPI(gs *models.GuideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guideID, err := strconv.Atoi(chi.URLParam(r, "guideID"))
		if err != nil {
			http.Error(w, "Invalid guide ID", http.StatusBadRequest)
			return
		}
		var req struct {
			Title            string `json:"title"`
			Slug             string `json:"slug"`
			Content          string `json:"content"`
			Description      string `json:"description"`
			FeaturedImageURL string `json:"featured_image_url"`
			IsPublished      *bool  `json:"is_published"`
			Categories       []int  `json:"categories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}
		existing, err := gs.GetByID(guideID)
		if err != nil {
			http.Error(w, "Guide not found", http.StatusNotFound)
			return
		}
		title := req.Title
		if title == "" {
			title = existing.Title
		}
		slug := req.Slug
		if slug == "" {
			slug = existing.Slug
		}
		content := req.Content
		if content == "" {
			content = existing.Content
		}
		description := req.Description
		if description == "" {
			description = existing.Description
		}
		featuredImageURL := req.FeaturedImageURL
		if featuredImageURL == "" {
			featuredImageURL = existing.FeaturedImageURL
		}
		isPublished := existing.IsPublished
		if req.IsPublished != nil {
			isPublished = *req.IsPublished
		}
		if err := gs.Update(guideID, title, slug, content, description, featuredImageURL, isPublished, req.Categories); err != nil {
			logger.Error().Err(err).Msg("error updating guide")
			http.Error(w, "Failed to update guide", http.StatusInternalServerError)
			return
		}
		updated, err := gs.GetByID(guideID)
		if err != nil {
			http.Error(w, "Failed to fetch updated guide", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, updated, http.StatusOK)
	}
}

func deleteGuideAPI(gs *models.GuideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guideID, err := strconv.Atoi(chi.URLParam(r, "guideID"))
		if err != nil {
			http.Error(w, "Invalid guide ID", http.StatusBadRequest)
			return
		}
		if err := gs.Delete(guideID); err != nil {
			logger.Error().Err(err).Msg("error deleting guide")
			http.Error(w, "Failed to delete guide", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// jsonResponse sends a JSON response with the given data and status code.
// ── Book API handlers ────────────────────────────────────────────────────────

func listBooksAPI(bs *models.BookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		books, err := bs.GetAllBooks()
		if err != nil {
			http.Error(w, "Failed to fetch books", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, books, http.StatusOK)
	}
}

func getBookAPI(bs *models.BookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "bookID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid book ID", http.StatusBadRequest)
			return
		}
		book, err := bs.GetByID(id)
		if err != nil {
			http.Error(w, "Book not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, book, http.StatusOK)
	}
}

func createBookAPI(bs *models.BookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID        int    `json:"user_id"`
			Title         string `json:"title"`
			Slug          string `json:"slug"`
			BookAuthor    string `json:"book_author"`
			ISBN          string `json:"isbn"`
			Publisher     string `json:"publisher"`
			PageCount     int    `json:"page_count"`
			CoverImageURL string `json:"cover_image_url"`
			Content       string `json:"content"`
			Description   string `json:"description"`
			MyThoughts    string `json:"my_thoughts"`
			LinkURL       string `json:"link_url"`
			ReadingStatus string `json:"reading_status"`
			Rating        float64 `json:"rating"`
			Medium        string  `json:"medium"`
			EbookReader   string  `json:"ebook_reader"`
			DateStarted   string `json:"date_started"`
			DateFinished  string `json:"date_finished"`
			IsPublished   bool   `json:"is_published"`
			Genres        []int  `json:"genres"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, "Title required", http.StatusBadRequest)
			return
		}
		book, err := bs.Create(req.UserID, req.Title, req.Slug, req.BookAuthor,
			req.ISBN, req.Publisher, req.PageCount, req.CoverImageURL,
			req.Content, req.Description, req.MyThoughts, req.LinkURL,
			req.ReadingStatus, req.Rating, req.Medium, req.EbookReader,
			req.DateStarted, req.DateFinished,
			req.IsPublished, req.Genres)
		if err != nil {
			http.Error(w, "Failed to create book: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, book, http.StatusCreated)
	}
}

func updateBookAPI(bs *models.BookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "bookID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid book ID", http.StatusBadRequest)
			return
		}
		var req struct {
			Title         string `json:"title"`
			Slug          string `json:"slug"`
			BookAuthor    string `json:"book_author"`
			ISBN          string `json:"isbn"`
			Publisher     string `json:"publisher"`
			PageCount     int    `json:"page_count"`
			CoverImageURL string `json:"cover_image_url"`
			Content       string `json:"content"`
			Description   string `json:"description"`
			MyThoughts    string `json:"my_thoughts"`
			LinkURL       string `json:"link_url"`
			ReadingStatus string `json:"reading_status"`
			Rating        float64 `json:"rating"`
			Medium        string  `json:"medium"`
			EbookReader   string  `json:"ebook_reader"`
			DateStarted   string `json:"date_started"`
			DateFinished  string `json:"date_finished"`
			IsPublished   bool   `json:"is_published"`
			Genres        []int  `json:"genres"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := bs.Update(id, req.Title, req.Slug, req.BookAuthor,
			req.ISBN, req.Publisher, req.PageCount, req.CoverImageURL,
			req.Content, req.Description, req.MyThoughts, req.LinkURL,
			req.ReadingStatus, req.Rating, req.Medium, req.EbookReader,
			req.DateStarted, req.DateFinished,
			req.IsPublished, req.Genres); err != nil {
			http.Error(w, "Failed to update book: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
	}
}

func deleteBookAPI(bs *models.BookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "bookID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid book ID", http.StatusBadRequest)
			return
		}
		if err := bs.Delete(id); err != nil {
			http.Error(w, "Failed to delete book: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
	}
}

// ── Book Genre API handlers ─────────────────────────────────────────────────

func listBookGenresAPI(bgs *models.BookGenreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		genres, err := bgs.GetAll()
		if err != nil {
			http.Error(w, "Failed to fetch genres", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, genres, http.StatusOK)
	}
}

func createBookGenreAPI(bgs *models.BookGenreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		genre, err := bgs.Create(req.Name, req.Group)
		if err != nil {
			http.Error(w, "Failed to create genre: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, genre, http.StatusCreated)
	}
}

func updateBookGenreAPI(bgs *models.BookGenreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "genreID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid genre ID", http.StatusBadRequest)
			return
		}
		var req struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		genre, err := bgs.Update(id, req.Name, req.Group)
		if err != nil {
			http.Error(w, "Failed to update genre: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, genre, http.StatusOK)
	}
}

func deleteBookGenreAPI(bgs *models.BookGenreService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "genreID")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid genre ID", http.StatusBadRequest)
			return
		}
		if err := bgs.Delete(id); err != nil {
			http.Error(w, "Failed to delete genre: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
	}
}

func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set(headerContentType, "application/json")
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

		w.Header().Set(headerContentType, "application/rss+xml; charset=utf-8")
		w.Header().Set(headerCacheCtrl, "public, max-age=300")

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

// sitemapDate normalises any date string to ISO 8601 (YYYY-MM-DD) for sitemap
// <lastmod>. Handles RFC3339, RFC3339Nano, human "January 2, 2006", and bare
// "2006-01-02". Falls back to the raw string if parsing fails (should not happen).
func sitemapDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Format("2006-01-02")
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s // fallback: return as-is
}

// robotsTxtHandler returns the robots.txt content for crawlers.
func robotsTxtHandler() http.HandlerFunc {
	body := `User-agent: *
Allow: /
Disallow: /admin/
Disallow: /api/
Disallow: /signin
Disallow: /signup
Disallow: /draw/*/edit

Sitemap: https://anshumanbiswas.com/sitemap.xml

# AI Crawlers Welcome
User-agent: GPTBot
Allow: /

User-agent: ClaudeBot
Allow: /

User-agent: Google-Extended
Allow: /

User-agent: PerplexityBot
Allow: /

User-agent: Amazonbot
Allow: /
`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, "text/plain; charset=utf-8")
		w.Header().Set(headerCacheCtrl, "public, max-age=86400")
		w.Write([]byte(body))
	}
}

// sitemapHandler dynamically generates an XML sitemap from published content.
func sitemapHandler(
	ps *models.PostService,
	ss *models.SlideService,
	gs *models.GuideService,
	bs *models.BookService,
	cs *models.CategoryService,
	bgs *models.BookGenreService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const baseURL = "https://anshumanbiswas.com"
		now := time.Now().Format("2006-01-02")

		w.Header().Set(headerContentType, "application/xml; charset=utf-8")
		w.Header().Set(headerCacheCtrl, "public, max-age=3600")

		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)
		// Home page
		fmt.Fprintf(w, "  <url><loc>%s/</loc><lastmod>%s</lastmod><priority>1.0</priority></url>\n", baseURL, now)

		// Static pages
		for _, page := range []struct {
			path     string
			priority string
		}{
			{"/about", "0.6"},
			{"/guides", "0.7"},
			{"/slides", "0.7"},
			{"/books", "0.7"},
		} {
			fmt.Fprintf(w, "  <url><loc>%s%s</loc><lastmod>%s</lastmod><priority>%s</priority></url>\n",
				baseURL, page.path, now, page.priority)
		}

		// Published blog posts
		if posts, err := ps.GetTopPostsWithPagination(1000, 0); err == nil && posts != nil {
			for _, p := range posts.Posts {
				if !p.IsPublished {
					continue
				}
				lastmod := p.CreatedAt
				if p.LastEditDate != "" {
					lastmod = p.LastEditDate
				}
				fmt.Fprintf(w, "  <url><loc>%s/blog/%s</loc><lastmod>%s</lastmod><priority>0.8</priority></url>\n",
					baseURL, xmlEscape(p.Slug), sitemapDate(lastmod))
			}
		}

		// Published slides
		if slides, err := ss.GetPublishedSlides(); err == nil && slides != nil {
			for _, s := range slides.Slides {
				lastmod := s.CreatedAt
				if s.UpdatedAt != "" {
					lastmod = s.UpdatedAt
				}
				fmt.Fprintf(w, "  <url><loc>%s/slides/%s</loc><lastmod>%s</lastmod><priority>0.7</priority></url>\n",
					baseURL, xmlEscape(s.Slug), sitemapDate(lastmod))
			}
		}

		// Published guides
		if guides, err := gs.GetPublishedGuides(); err == nil && guides != nil {
			for _, g := range guides.Guides {
				lastmod := g.CreatedAt
				if g.UpdatedAt != "" {
					lastmod = g.UpdatedAt
				}
				fmt.Fprintf(w, "  <url><loc>%s/guides/%s</loc><lastmod>%s</lastmod><priority>0.8</priority></url>\n",
					baseURL, xmlEscape(g.Slug), sitemapDate(lastmod))
			}
		}

		// Tag pages — URL-encode names (spaces → %20)
		if cats, err := cs.GetAll(); err == nil {
			for _, c := range cats {
				encodedName := url.PathEscape(c.Name)
				fmt.Fprintf(w, "  <url><loc>%s/tags/%s</loc><lastmod>%s</lastmod><priority>0.5</priority></url>\n",
					baseURL, encodedName, now)
			}
		}

		// Published books
		if books, err := bs.GetPublishedBooks(); err == nil && books != nil {
			for _, b := range books.Books {
				lastmod := b.CreatedAt
				if b.UpdatedAt != "" {
					lastmod = b.UpdatedAt
				}
				fmt.Fprintf(w, "  <url><loc>%s/books/%s</loc><lastmod>%s</lastmod><priority>0.6</priority></url>\n",
					baseURL, xmlEscape(b.Slug), sitemapDate(lastmod))
			}
		}

		// Book genre pages
		if genres, err := bgs.GetAll(); err == nil {
			for _, g := range genres {
				encodedName := url.PathEscape(g.Name)
				fmt.Fprintf(w, "  <url><loc>%s/books/genre/%s</loc><lastmod>%s</lastmod><priority>0.4</priority></url>\n",
					baseURL, encodedName, now)
			}
		}

		fmt.Fprint(w, "</urlset>\n")
	}
}

// crawlerStatsHandler returns crawler analytics data (admin-only).
func crawlerStatsHandler(analyticsService *models.AnalyticsService, sessionService *models.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := utils.IsUserLoggedIn(r, sessionService)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !models.CanViewAdminPanel(user.Role) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		period := r.URL.Query().Get("period")
		if period == "" {
			period = "30d"
		}

		stats, err := analyticsService.GetCrawlerStats(period)
		if err != nil {
			http.Error(w, "Failed to get crawler stats", http.StatusInternalServerError)
			return
		}

		w.Header().Set(headerContentType, "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// crawlerRulesListHandler returns all crawler rules (admin-only).
func crawlerRulesListHandler(service *models.CrawlerRuleService, sessionService *models.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := utils.IsUserLoggedIn(r, sessionService)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !models.CanViewAdminPanel(user.Role) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		rules, err := service.GetAll()
		if err != nil {
			http.Error(w, "Failed to get crawler rules", http.StatusInternalServerError)
			return
		}
		if rules == nil {
			rules = []models.CrawlerRule{}
		}

		w.Header().Set(headerContentType, "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

// crawlerRulesCreateHandler creates a new crawler rule (admin-only).
func crawlerRulesCreateHandler(service *models.CrawlerRuleService, sessionService *models.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := utils.IsUserLoggedIn(r, sessionService)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !models.CanViewAdminPanel(user.Role) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		var body struct {
			CrawlerPattern string `json:"crawler_pattern"`
			Action         string `json:"action"`
			TimeStart      *int   `json:"time_start"`
			TimeEnd        *int   `json:"time_end"`
			Reason         string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.CrawlerPattern == "" {
			http.Error(w, "crawler_pattern is required", http.StatusBadRequest)
			return
		}
		if body.Action == "" {
			body.Action = "allow"
		}
		if body.Action != "allow" && body.Action != "block" && body.Action != "time_restrict" {
			http.Error(w, "action must be allow, block, or time_restrict", http.StatusBadRequest)
			return
		}

		if err := service.Create(body.CrawlerPattern, body.Action, body.TimeStart, body.TimeEnd, body.Reason); err != nil {
			http.Error(w, "Failed to create rule", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set(headerContentType, "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}
}

// crawlerRulesUpdateHandler updates an existing crawler rule (admin-only).
func crawlerRulesUpdateHandler(service *models.CrawlerRuleService, sessionService *models.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := utils.IsUserLoggedIn(r, sessionService)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !models.CanViewAdminPanel(user.Role) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var body struct {
			Action    string `json:"action"`
			TimeStart *int   `json:"time_start"`
			TimeEnd   *int   `json:"time_end"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Action != "allow" && body.Action != "block" && body.Action != "time_restrict" {
			http.Error(w, "action must be allow, block, or time_restrict", http.StatusBadRequest)
			return
		}

		if err := service.Update(id, body.Action, body.TimeStart, body.TimeEnd, body.Reason); err != nil {
			http.Error(w, "Failed to update rule", http.StatusInternalServerError)
			return
		}

		w.Header().Set(headerContentType, "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}
}

// crawlerRulesDeleteHandler deletes a crawler rule (admin-only).
func crawlerRulesDeleteHandler(service *models.CrawlerRuleService, sessionService *models.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := utils.IsUserLoggedIn(r, sessionService)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !models.CanViewAdminPanel(user.Role) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		if err := service.Delete(id); err != nil {
			http.Error(w, "Failed to delete rule", http.StatusInternalServerError)
			return
		}

		w.Header().Set(headerContentType, "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}
}
