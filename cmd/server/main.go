package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"courses-hub/internal/db"
	"courses-hub/internal/handlers"
)

func main() {
	addr := getenv("ADDR", ":8080")
	webDir := getenv("WEB_DIR", "./web")
	dbPath := getenv("DB_PATH", "./data/courses.db")
	coursesDir := getenv("COURSES_DIR", "./data/courses")
	maxUploadMB := getenvInt("MAX_UPLOAD_MB", 50)

	for _, dir := range []string{filepath.Dir(dbPath), coursesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create dir %s: %v", dir, err)
		}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	mux := http.NewServeMux()

	// API + course pages (registered before the catch-all portal handler).
	handlers.New(database, coursesDir, int64(maxUploadMB)<<20).Register(mux)

	// Liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	// Portal (list of courses, tracking.js) served at the root.
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join(webDir, "portal"))))

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shut down cleanly on Ctrl+C / `docker stop` so SQLite closes properly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("courses-hub listening on %s (courses: %s, db: %s)", addr, coursesDir, dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		log.Printf("invalid %s=%q, using %d", key, v, fallback)
	}
	return fallback
}

// logRequests is a tiny logging middleware so I can see traffic.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
