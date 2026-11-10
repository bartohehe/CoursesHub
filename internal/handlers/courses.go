package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"courses-hub/internal/db"
)

// Handlers holds dependencies shared by the HTTP handlers.
type Handlers struct {
	DB             *db.DB
	CoursesDir     string // where uploaded course files live (one dir per slug)
	MaxUploadBytes int64
}

// New creates a Handlers value.
func New(database *db.DB, coursesDir string, maxUploadBytes int64) *Handlers {
	return &Handlers{DB: database, CoursesDir: coursesDir, MaxUploadBytes: maxUploadBytes}
}

// Register wires the API and course routes onto mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/courses", h.ListCourses)
	mux.HandleFunc("POST /api/courses", h.CreateCourse)
	mux.HandleFunc("DELETE /api/courses/{slug}", h.DeleteCourse)
	mux.HandleFunc("POST /api/progress", h.UpdateProgress)

	// Course entry page, served with the tracking script injected.
	mux.HandleFunc("GET /courses/{slug}/{$}", h.ServeCourse)
	mux.HandleFunc("GET /courses/{slug}/index.html", h.ServeCourse)
	// Any other files inside a course directory (images etc.).
	files := http.StripPrefix("/courses/", http.FileServer(http.Dir(h.CoursesDir)))
	mux.Handle("GET /courses/", noDirListing(files))
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validSlug(s string) bool {
	return len(s) <= 64 && slugRe.MatchString(s)
}

// ListCourses handles GET /api/courses and returns all courses with status.
func (h *Handlers) ListCourses(w http.ResponseWriter, r *http.Request) {
	courses, err := h.DB.ListCourses()
	if err != nil {
		log.Printf("ListCourses: %v", err)
		writeError(w, http.StatusInternalServerError, "nie udało się wczytać kursów")
		return
	}
	if courses == nil {
		courses = []db.Course{}
	}
	writeJSON(w, http.StatusOK, courses)
}

// CreateCourse handles POST /api/courses (multipart form): it stores the
// uploaded HTML file as <CoursesDir>/<slug>/index.html and registers the course.
func (h *Handlers) CreateCourse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("plik jest za duży (limit %d MB)", h.MaxUploadBytes>>20))
			return
		}
		writeError(w, http.StatusBadRequest, "niepoprawny formularz")
		return
	}
	defer r.MultipartForm.RemoveAll()

	in := db.NewCourse{
		Slug:        strings.TrimSpace(r.FormValue("slug")),
		Title:       strings.TrimSpace(r.FormValue("title")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Category:    strings.TrimSpace(r.FormValue("category")),
	}
	switch {
	case in.Title == "" || len(in.Title) > 200:
		writeError(w, http.StatusBadRequest, "tytuł jest wymagany (max 200 znaków)")
		return
	case !validSlug(in.Slug):
		writeError(w, http.StatusBadRequest, "slug: tylko małe litery, cyfry i myślniki (np. web-scraping)")
		return
	case len(in.Description) > 1000 || len(in.Category) > 50:
		writeError(w, http.StatusBadRequest, "opis lub kategoria są za długie")
		return
	}
	if in.Category == "" {
		in.Category = "inne"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "wybierz plik HTML kursu")
		return
	}
	defer file.Close()
	if ext := strings.ToLower(filepath.Ext(header.Filename)); ext != ".html" && ext != ".htm" {
		writeError(w, http.StatusBadRequest, "plik kursu musi mieć rozszerzenie .html")
		return
	}

	exists, err := h.DB.CourseExists(in.Slug)
	if err != nil {
		log.Printf("CreateCourse: %v", err)
		writeError(w, http.StatusInternalServerError, "błąd bazy danych")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "kurs o tym slugu już istnieje")
		return
	}

	dir := filepath.Join(h.CoursesDir, in.Slug)
	// A directory without a DB row is a leftover from a failed upload — replace it.
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("CreateCourse: clean %s: %v", dir, err)
	}
	if err := saveFile(filepath.Join(dir, "index.html"), file); err != nil {
		os.RemoveAll(dir)
		log.Printf("CreateCourse: save: %v", err)
		writeError(w, http.StatusInternalServerError, "nie udało się zapisać pliku")
		return
	}

	course, err := h.DB.CreateCourse(in)
	if err != nil {
		os.RemoveAll(dir)
		if errors.Is(err, db.ErrSlugTaken) {
			writeError(w, http.StatusConflict, "kurs o tym slugu już istnieje")
			return
		}
		log.Printf("CreateCourse: %v", err)
		writeError(w, http.StatusInternalServerError, "nie udało się zapisać kursu")
		return
	}
	writeJSON(w, http.StatusCreated, course)
}

// DeleteCourse handles DELETE /api/courses/{slug}: removes the DB rows and files.
func (h *Handlers) DeleteCourse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !validSlug(slug) {
		writeError(w, http.StatusNotFound, "nie ma takiego kursu")
		return
	}
	if err := h.DB.DeleteCourse(slug); err != nil {
		if errors.Is(err, db.ErrCourseNotFound) {
			writeError(w, http.StatusNotFound, "nie ma takiego kursu")
			return
		}
		log.Printf("DeleteCourse: %v", err)
		writeError(w, http.StatusInternalServerError, "nie udało się usunąć kursu")
		return
	}
	if err := os.RemoveAll(filepath.Join(h.CoursesDir, slug)); err != nil {
		log.Printf("DeleteCourse: remove files for %q: %v", slug, err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func saveFile(path string, src io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// noDirListing returns 404 for directory paths instead of an index listing.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isNotExist is a small helper so serve.go reads cleanly.
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
