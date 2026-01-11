package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
)

type MangaOption struct {
	ID   int
	Name string
}

func get_random_mangas_with_frequency(db *sql.DB, userId string) ([]MangaOption, error) {
	ids_query := `
		select m.id 
		from mangas m
		left join user_manga_stats ums on m.id = ums.manga_id and ums.user_id = ?
		where (ums.times_shown < 3 or ums.times_shown is null)
		and (ums.last_shown < datetime('now', '-30 minutes') or ums.last_shown is null)
		and exists (select 1 from images i where i.manga_id = m.id)
	`

	rows, err := db.Query(ids_query, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligible manga IDs: %w", err)
	}
	defer rows.Close()

	var eligibleIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		eligibleIDs = append(eligibleIDs, id)
	}

	// If not enough eligible mangas, fallback to any 4 random mangas
	if len(eligibleIDs) < 4 {
		fallbackQuery := `
			select distinct m.id, m.name 
			from mangas m
			inner join images i ON m.id = i.manga_id
			order by random()
			limit 4
		`

		rows, err := db.Query(fallbackQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to query fallback mangas: %w", err)
		}
		defer rows.Close()

		var mangas []MangaOption
		for rows.Next() {
			var manga MangaOption
			if err := rows.Scan(&manga.ID, &manga.Name); err != nil {
				continue
			}
			mangas = append(mangas, manga)
		}

		if len(mangas) < 4 {
			return nil, fmt.Errorf("not enough unique mangas found: got %d, need 4", len(mangas))
		}

		return mangas, nil
	}

	rand.Shuffle(len(eligibleIDs), func(i, j int) {
		eligibleIDs[i], eligibleIDs[j] = eligibleIDs[j], eligibleIDs[i]
	})

	selectedIDs := eligibleIDs[:4]

	placeholders := strings.Repeat("?,", 4)
	placeholders = placeholders[:len(placeholders)-1]

	details_query := fmt.Sprintf(`
		select m.id, m.name 
		from mangas m
		where m.id IN (%s)
	`, placeholders)

	args := make([]interface{}, 4)
	for i, id := range selectedIDs {
		args[i] = id
	}

	rows, err = db.Query(details_query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query manga details: %w", err)
	}
	defer rows.Close()

	var mangas []MangaOption
	for rows.Next() {
		var manga MangaOption
		if err := rows.Scan(&manga.ID, &manga.Name); err != nil {
			continue
		}
		mangas = append(mangas, manga)
	}

	return mangas, nil
}
