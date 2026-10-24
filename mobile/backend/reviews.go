package main

import (
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type Review struct {
	ID        int64  `json:"id"`
	PlaceID   int    `json:"placeId"`
	Author    string `json:"author"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type ReviewRequest struct {
	Author string `json:"author"`
	Rating int    `json:"rating"`
	Text   string `json:"text"`
}

type ratingInfo struct {
	avg   float64
	count int
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// одно соединение: sqlite не любит параллельную запись, а :memory: живёт в рамках соединения
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS reviews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		place_id INTEGER NOT NULL,
		author TEXT NOT NULL,
		rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
		text TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS reviews_place_idx ON reviews (place_id);`)
	return db, err
}

func validateReview(r ReviewRequest) []string {
	var reasons []string
	if strings.TrimSpace(r.Author) == "" || utf8.RuneCountInString(r.Author) > 50 {
		reasons = append(reasons, "author must be 1..50 characters")
	}
	if r.Rating < 1 || r.Rating > 5 {
		reasons = append(reasons, "rating must be 1..5")
	}
	if utf8.RuneCountInString(r.Text) > 1000 {
		reasons = append(reasons, "text must be at most 1000 characters")
	}
	return reasons
}

func addReview(db *sql.DB, placeID int, r ReviewRequest) (Review, error) {
	review := Review{
		PlaceID:   placeID,
		Author:    strings.TrimSpace(r.Author),
		Rating:    r.Rating,
		Text:      strings.TrimSpace(r.Text),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	res, err := db.Exec(`INSERT INTO reviews (place_id, author, rating, text, created_at) VALUES (?, ?, ?, ?, ?)`,
		review.PlaceID, review.Author, review.Rating, review.Text, review.CreatedAt)
	if err != nil {
		return review, err
	}
	review.ID, err = res.LastInsertId()
	return review, err
}

func getReviews(db *sql.DB, placeID int) ([]Review, error) {
	rows, err := db.Query(`SELECT id, place_id, author, rating, text, created_at FROM reviews
		WHERE place_id = ? ORDER BY id DESC`, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.PlaceID, &r.Author, &r.Rating, &r.Text, &r.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

// ratings returns average rating and number of reviews for every place that has reviews
func ratings(db *sql.DB) (map[int]ratingInfo, error) {
	rows, err := db.Query(`SELECT place_id, AVG(rating), COUNT(*) FROM reviews GROUP BY place_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]ratingInfo{}
	for rows.Next() {
		var id int
		var info ratingInfo
		if err := rows.Scan(&id, &info.avg, &info.count); err != nil {
			return nil, err
		}
		result[id] = info
	}
	return result, rows.Err()
}
