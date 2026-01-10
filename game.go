package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func populate_mangas() (map[string]string, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./manga_images.db"
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var mangaNames = map[string]string{}
	usedNames := make(map[string]bool)

	for range 4 {
		manga, err := get_random_manga(db)
		if err != nil {
			return nil, fmt.Errorf("failed to get random manga: %w", err)
		}

		for usedNames[manga.Name] {
			manga, err = get_random_manga(db)
			if err != nil {
				return nil, fmt.Errorf("failed to get random manga (retry): %w", err)
			}
		}

		usedNames[manga.Name] = true
		mangaNames[manga.Name] = fmt.Sprintf("%d", manga.ID)
	}

	for i, data := range mangaNames {
		fmt.Println(i, ":", data)
	}

	return mangaNames, nil
}

func get_random_image_url(mangaId string) (string, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./manga_images.db" // fallback to default
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var mangaName string
	err = db.QueryRow("select name from mangas where id = ?", mangaId).Scan(&mangaName)
	if err != nil {
		return "", fmt.Errorf("failed to get manga name: %w", err)
	}

	rows, err := db.Query("select image_name from images WHERE manga_id = ?", mangaId)
	if err != nil {
		return "", fmt.Errorf("failed to query images: %w", err)
	}
	defer rows.Close()

	var imageNames []string
	for rows.Next() {
		var imageName string
		if err := rows.Scan(&imageName); err != nil {
			continue
		}
		imageNames = append(imageNames, imageName)
	}

	if len(imageNames) == 0 {
		return "", fmt.Errorf("no images found for manga")
	}

	randomImageName := imageNames[rand.Intn(len(imageNames))]

	return fmt.Sprintf("https://node1.alejoseed.com/mangas/%s/%s", mangaName, randomImageName), nil
}

func find_mangas() (gin.H, error) {
	mangas, err := populate_mangas()
	if err != nil {
		return nil, fmt.Errorf("failed to populate mangas: %w", err)
	}

	var mangaNames = []string{}

	for key := range mangas {
		mangaNames = append(mangaNames, key)
	}

	var correctNum int = rand.Intn(4)

	mangaId := mangas[mangaNames[correctNum]]
	fmt.Print(mangaId)

	imageUrl, err := get_random_image_url(mangaId)
	if err != nil {
		return nil, fmt.Errorf("failed to get random image URL: %w", err)
	}

	response := gin.H{
		"mangas":               mangaNames,
		"CurrentStoredMangaId": mangaId,
		"index":                correctNum,
		"imageUrl":             imageUrl,
	}

	return response, nil
}

func find_image(mangaId string) (string, error) {
	return get_random_image_url(mangaId)
}
