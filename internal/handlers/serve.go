package handlers

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// ServeCourse serves a course's index.html with the tracking script injected
// right before </body>. The file on disk stays untouched.
func (h *Handlers) ServeCourse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !validSlug(slug) {
		http.NotFound(w, r)
		return
	}

	data, err := os.ReadFile(filepath.Join(h.CoursesDir, slug, "index.html"))
	if err != nil {
		if !isNotExist(err) {
			log.Printf("ServeCourse %q: %v", slug, err)
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(injectTracking(data, slug))
}

// injectTracking inserts the tracking <script> before the last </body> tag,
// or appends it when the document has none. slug is pre-validated, so it is
// safe inside the attribute.
func injectTracking(html []byte, slug string) []byte {
	tag := []byte(`<script src="/tracking.js" data-course-slug="` + slug + `"></script>`)
	i := lastIndexFold(html, []byte("</body>"))
	if i < 0 {
		return append(html, tag...)
	}
	out := make([]byte, 0, len(html)+len(tag))
	out = append(out, html[:i]...)
	out = append(out, tag...)
	return append(out, html[i:]...)
}

// lastIndexFold is bytes.LastIndex with ASCII case-insensitive matching.
func lastIndexFold(s, sep []byte) int {
	for i := len(s) - len(sep); i >= 0; i-- {
		if bytes.EqualFold(s[i:i+len(sep)], sep) {
			return i
		}
	}
	return -1
}
