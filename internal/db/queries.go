package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Course is a course row joined with its progress status.
type Course struct {
	ID            int64      `json:"id"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Path          string     `json:"path"`
	Category      string     `json:"category"`
	Status        string     `json:"status"`
	LastVisitedAt *time.Time `json:"last_visited_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

// Valid progress statuses.
const (
	StatusNotStarted = "not_started"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

var (
	// ErrCourseNotFound is returned when a slug does not match any course.
	ErrCourseNotFound = errors.New("course not found")
	// ErrSlugTaken is returned when creating a course with an existing slug.
	ErrSlugTaken = errors.New("slug already exists")
)

// NewCourse holds the fields needed to register a course.
type NewCourse struct {
	Slug        string
	Title       string
	Description string
	Category    string
}

// CoursePath returns the public URL of a course's entry page.
func CoursePath(slug string) string {
	return "/courses/" + slug + "/"
}

// CourseExists reports whether a course with the given slug is registered.
func (d *DB) CourseExists(slug string) (bool, error) {
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM courses WHERE slug = ?`, slug).Scan(&n); err != nil {
		return false, fmt.Errorf("check slug %q: %w", slug, err)
	}
	return n > 0, nil
}

// CreateCourse inserts a new course and returns it with "not_started" status.
func (d *DB) CreateCourse(c NewCourse) (Course, error) {
	path := CoursePath(c.Slug)
	res, err := d.Exec(`INSERT INTO courses (slug, title, description, path, category) VALUES (?, ?, ?, ?, ?)`,
		c.Slug, c.Title, c.Description, path, c.Category)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return Course{}, ErrSlugTaken
		}
		return Course{}, fmt.Errorf("insert course %q: %w", c.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Course{}, fmt.Errorf("course id: %w", err)
	}
	return Course{
		ID:          id,
		Slug:        c.Slug,
		Title:       c.Title,
		Description: c.Description,
		Path:        path,
		Category:    c.Category,
		Status:      StatusNotStarted,
	}, nil
}

// DeleteCourse removes a course and its progress row.
func (d *DB) DeleteCourse(slug string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow(`SELECT id FROM courses WHERE slug = ?`, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCourseNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup slug %q: %w", slug, err)
	}
	if _, err := tx.Exec(`DELETE FROM progress WHERE course_id = ?`, id); err != nil {
		return fmt.Errorf("delete progress: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM courses WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete course: %w", err)
	}
	return tx.Commit()
}

// ValidStatus reports whether s is an allowed progress status.
func ValidStatus(s string) bool {
	switch s {
	case StatusNotStarted, StatusInProgress, StatusCompleted:
		return true
	default:
		return false
	}
}

// ListCourses returns all courses with their progress status. Courses without a
// progress row default to "not_started".
func (d *DB) ListCourses() ([]Course, error) {
	rows, err := d.Query(`
		SELECT c.id, c.slug, c.title, COALESCE(c.description, ''), c.path, COALESCE(c.category, ''),
		       COALESCE(p.status, 'not_started'), p.last_visited_at, p.completed_at
		FROM courses c
		LEFT JOIN progress p ON p.course_id = c.id
		ORDER BY c.category, c.title COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()

	var courses []Course
	for rows.Next() {
		var c Course
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Path, &c.Category,
			&c.Status, &c.LastVisitedAt, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}

// UpsertProgress sets the progress status for the course identified by slug.
// last_visited_at is always bumped to now; completed_at is set to now when the
// status becomes "completed" and cleared otherwise.
func (d *DB) UpsertProgress(slug, status string) error {
	var courseID int64
	err := d.QueryRow(`SELECT id FROM courses WHERE slug = ?`, slug).Scan(&courseID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCourseNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup slug %q: %w", slug, err)
	}

	now := time.Now().UTC()
	var completedAt *time.Time
	if status == StatusCompleted {
		completedAt = &now
	}

	_, err = d.Exec(`
		INSERT INTO progress (course_id, status, last_visited_at, completed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(course_id) DO UPDATE SET
			status          = excluded.status,
			last_visited_at = excluded.last_visited_at,
			completed_at    = excluded.completed_at`,
		courseID, status, now, completedAt)
	if err != nil {
		return fmt.Errorf("upsert progress for %q: %w", slug, err)
	}
	return nil
}
