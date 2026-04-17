package main

import (
	"context"
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
	pagesHandler := pages.NewHandler(pagesSvc)

	imagesSvc := images.NewService(cfg.UploadsDir)
	imagesHandler := images.NewHandler(imagesSvc)

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
	mux.HandleFunc("GET /{path...}", staticHandler(cfg.DistDir))

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server stopped")
}

func staticHandler(distDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/" {
			urlPath = "/index.html"
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

		// Try index.html in subdirectory
		indexPath := filepath.Join(distDir, filepath.FromSlash(urlPath), "index.html")
		if serveIfExists(w, r, indexPath) {
			return
		}

		// Serve catch-all SPA shell
		spaPath := filepath.Join(distDir, "[[...slug]].html")
		if serveIfExists(w, r, spaPath) {
			return
		}

		// Final fallback
		fallback := filepath.Join(distDir, "index.html")
		http.ServeFile(w, r, fallback)
	}
}

func serveIfExists(w http.ResponseWriter, r *http.Request, path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeFile(w, r, path)
	return true
}
