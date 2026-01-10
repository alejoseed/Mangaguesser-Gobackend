package main

import (
	"database/sql"
	"fmt"
	"math/rand"
)

func get_random_manga(db *sql.DB) (struct {
	ID   int
	Name string
}, error) {
	var count int
	err := db.QueryRow("select count(*) from mangas").Scan(&count)
	if err != nil {
		return struct {
			ID   int
			Name string
		}{}, fmt.Errorf("failed to count manga: %w", err)
	}

	if count == 0 {
		return struct {
			ID   int
			Name string
		}{}, fmt.Errorf("no manga found in database")
	}

	offset := rand.Intn(count)

	var manga struct {
		ID   int
		Name string
	}

	query := "select id, name from mangas limit 1 offset ?"
	err = db.QueryRow(query, offset).Scan(&manga.ID, &manga.Name)
	if err != nil {
		return struct {
			ID   int
			Name string
		}{}, fmt.Errorf("failed to fetch random manga: %w", err)
	}

	return manga, nil
}
