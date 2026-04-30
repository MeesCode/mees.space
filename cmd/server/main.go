package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"mees.space/internal/ai"
	"mees.space/internal/auth"
	"mees.space/internal/config"
	"mees.space/internal/database"
	"mees.space/internal/folders"
	"mees.space/internal/images"
	"mees.space/internal/middleware"
	"mees.space/internal/pages"
	"mees.space/internal/render"
	"mees.space/internal/seo"
	"mees.space/internal/settings"
)

func main() {
	_ = godotenv.Load() // load .env if present

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Ensure directories exist
	os.MkdirAll(cfg.ContentDir, 0755)
	os.MkdirAll(cfg.UploadsDir, 0755)
	os.MkdirAll(cfg.DistDir, 0755)

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal("database:", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatal("migrations:", err)
	}

	if err := auth.SeedAdmin(db, cfg.AdminPassword); err != nil {
		log.Fatal("seed admin:", err)
	}

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiryMinutes, cfg.JWTRefreshExpiryHrs)
	authHandler := auth.NewHandler(db, jwtSvc)

	pagesSvc := pages.NewService(db, cfg.ContentDir)
	descGen := pages.NewGenerator(db, 10*time.Second)
	pagesHandler := pages.NewHandler(pagesSvc, cfg.BaseURL, descGen)

	injector, injErr := seo.NewInjector(cfg.DistDir)
	if injErr != nil {
		log.Fatal("seo injector:", injErr)
	}

	renderer := render.New()

	backfillCtx, backfillCancel := context.WithCancel(context.Background())
	go descGen.BackfillEmpty(backfillCtx, pagesSvc)

	imagesSvc := images.NewService(cfg.UploadsDir)
	imagesHandler := images.NewHandler(imagesSvc, cfg.ContentDir)

	foldersHandler := folders.NewHandler(cfg.ContentDir, db)

	settingsHandler := settings.NewHandler(db)
	aiHandler := ai.NewHandler(db)

	protected := func(next http.HandlerFunc) http.Handler {
		return auth.RequireAuth(jwtSvc, next)
	}

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)

	// Pages (public, with optional auth to handle draft visibility)
	optionalAuth := func(next http.HandlerFunc) http.Handler {
		return auth.OptionalAuth(jwtSvc, next)
	}
	mux.Handle("GET /api/pages/tree", optionalAuth(pagesHandler.GetTree))
	mux.Handle("GET /api/pages/{path...}", optionalAuth(pagesHandler.GetPage))
	mux.HandleFunc("GET /feed.xml", pagesHandler.GetRSS)
	mux.HandleFunc("GET /sitemap.xml", pagesHandler.GetSitemap)
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nDisallow: /api/\nAllow: /\n\nSitemap: %s/sitemap.xml\n", cfg.BaseURL)
	})

	// View count (separate route since wildcard must be at end)
	mux.HandleFunc("POST /api/views/{path...}", pagesHandler.IncrementView)

	// Pages (protected)
	mux.Handle("POST /api/pages/{path...}", protected(pagesHandler.CreatePage))
	mux.Handle("PUT /api/pages/{path...}", protected(pagesHandler.UpdatePage))
	mux.Handle("PATCH /api/pages/{path...}", protected(pagesHandler.RenamePage))
	mux.Handle("DELETE /api/pages/{path...}", protected(pagesHandler.DeletePage))

	// Folders (protected)
	mux.Handle("POST /api/folders/{path...}", protected(foldersHandler.Create))
	mux.Handle("PUT /api/folders/{path...}", protected(foldersHandler.Rename))
	mux.Handle("DELETE /api/folders/{path...}", protected(foldersHandler.Delete))

	// Images
	mux.Handle("GET /api/images", protected(imagesHandler.List))
	mux.Handle("POST /api/images", protected(imagesHandler.Upload))
	mux.Handle("DELETE /api/images/{filename}", protected(imagesHandler.Delete))

	// Settings (protected)
	mux.Handle("GET /api/settings", protected(settingsHandler.Get))
	mux.Handle("PUT /api/settings", protected(settingsHandler.Update))

	// AI (protected)
	mux.Handle("POST /api/ai/complete", protected(aiHandler.Complete))

	// Uploaded images (public)
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Admin redirect
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/editor", http.StatusFound)
	})

	// Catch-all: serve Next.js static export
	mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir, cfg.BaseURL, pagesSvc, injector, renderer))

	handler := middleware.SecurityHeaders(middleware.Logger(mux))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	go func() {
		log.Printf("Starting server on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	backfillCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server stopped")
}

func staticHandler(distDir, baseURL string, pagesSvc *pages.Service, injector *seo.Injector, renderer *render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/" {
			serveContentPage(w, r, "", baseURL, pagesSvc, injector, renderer)
			return
		}

		// Try exact file
		filePath := filepath.Join(distDir, filepath.FromSlash(urlPath))
		if serveIfExists(w, r, filePath) {
			return
		}

		// Try .html extension
		if !strings.HasSuffix(urlPath, ".html") {
			htmlPath := filepath.Join(distDir, filepath.FromSlash(urlPath+".html"))
			if serveIfExists(w, r, htmlPath) {
				return
			}
		}

		// Try index.html in subdirectory (e.g., /admin/editor/)
		indexPath := filepath.Join(distDir, filepath.FromSlash(urlPath), "index.html")
		if serveIfExists(w, r, indexPath) {
			return
		}

		// Admin paths fall back to raw shell (don't inject SEO)
		if strings.HasPrefix(urlPath, "/admin") {
			writeHTML(w, injector.Raw())
			return
		}

		// Paths with a file extension (other than .html) that didn't match any
		// static file are missing assets — return a proper 404 rather than
		// serving the SPA shell as a soft 404.
		if ext := filepath.Ext(urlPath); ext != "" && ext != ".html" {
			http.NotFound(w, r)
			return
		}

		// Content page fallback: strip leading slash and attempt lookup.
		pagePath := strings.TrimPrefix(urlPath, "/")
		serveContentPage(w, r, pagePath, baseURL, pagesSvc, injector, renderer)
	}
}

func serveContentPage(w http.ResponseWriter, r *http.Request, pagePath, baseURL string, pagesSvc *pages.Service, injector *seo.Injector, renderer *render.Renderer) {
	if pagePath == "" {
		pagePath = "home"
	}

	page, err := pagesSvc.GetPage(pagePath)
	if err != nil {
		meta := seo.PageMeta{
			Title:       "Not Found — Mees Brinkhuis",
			Description: "",
			NoIndex:     true,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(injector.Inject(meta, seo.BodyInjection{}))
		return
	}

	canonical := baseURL + "/" + pagePath
	if pagePath == "home" {
		canonical = baseURL
	}

	meta := seo.PageMeta{
		Title:        page.Title,
		Description:  page.Description,
		CanonicalURL: canonical,
		OGImage:      baseURL + "/mees.png",
		NoIndex:      !page.Published,
	}

	body := seo.BodyInjection{}
	if page.Published {
		htmlBytes, renderErr := renderer.ToHTML([]byte(page.Content))
		if renderErr != nil {
			log.Printf("ssr: render %s: %v", pagePath, renderErr)
		} else {
			body.HTML = htmlBytes
			bootstrap := *page
			bootstrap.RenderedHTML = string(htmlBytes)
			if jsonBytes, jsonErr := json.Marshal(bootstrap); jsonErr != nil {
				log.Printf("ssr: marshal %s: %v", pagePath, jsonErr)
			} else {
				body.Data = jsonBytes
			}
		}
	} else {
		// Don't leak draft details to crawlers or in the bootstrap payload.
		meta.Title = "Draft — Mees Brinkhuis"
		meta.Description = ""
	}

	writeHTML(w, injector.Inject(meta, body))
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(body)
}

func serveIfExists(w http.ResponseWriter, r *http.Request, path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeFile(w, r, path)
	return true
}
