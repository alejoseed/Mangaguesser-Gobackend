package main

import (
	"fmt"
	"math/rand"

	"github.com/gin-gonic/gin"
)

func populate_mangas_with_frequency(userId string) (map[string]string, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	mangaOptions, err := get_random_mangas_with_frequency(DB, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get random mangas: %w", err)
	}

	mangaNames := map[string]string{}
	for _, manga := range mangaOptions {
		mangaNames[manga.Name] = fmt.Sprintf("%d", manga.ID)
	}

	return mangaNames, nil
}

func get_random_image_url(userId string, mangaId string) (string, error) {
	if DB == nil {
		return "", fmt.Errorf("database not initialized")
	}

	var mangaName string
	err := DB.QueryRow("select name from mangas where id = ?", mangaId).Scan(&mangaName)
	if err != nil {
		return "", fmt.Errorf("failed to get manga name: %w", err)
	}

	image_ids_query := `
		select i.image_name 
		from images i
		left join user_manga_history umh on i.image_name = umh.image_name
			and umh.user_id = ? and umh.manga_id = ?
		where i.manga_id = ?
		and (umh.image_name is null or umh.view_count < 1)
	`

	rows, err := DB.Query(image_ids_query, userId, mangaId, mangaId)
	if err != nil {
		return "", fmt.Errorf("failed to query eligible image names: %w", err)
	}
	defer rows.Close()

	var eligibleImages []string
	for rows.Next() {
		var imageName string
		if err := rows.Scan(&imageName); err != nil {
			continue
		}
		eligibleImages = append(eligibleImages, imageName)
	}

	if len(eligibleImages) == 0 {
		fallbackQuery := `
			select image_name from images 
			where manga_id = ? 
			order by random() limit 1
		`

		var fallbackImage string
		err = DB.QueryRow(fallbackQuery, mangaId).Scan(&fallbackImage)
		if err != nil {
			return "", fmt.Errorf("no images found for manga: %w", err)
		}

		if err := trackImageViewed(userId, mangaId, fallbackImage); err != nil {
			fmt.Printf("Failed to track fallback image view: %v\n", err)
		}

		return fmt.Sprintf("https://node1.alejoseed.com/mangas/%s/%s", mangaName, fallbackImage), nil
	}

	rand.Shuffle(len(eligibleImages), func(i, j int) {
		eligibleImages[i], eligibleImages[j] = eligibleImages[j], eligibleImages[i]
	})

	selectedImage := eligibleImages[0]

	if err := trackImageViewed(userId, mangaId, selectedImage); err != nil {
		fmt.Printf("Failed to track image view: %v\n", err)
	}

	return fmt.Sprintf("https://node1.alejoseed.com/mangas/%s/%s", mangaName, selectedImage), nil
}

func find_mangas_with_frequency(userId string) (gin.H, error) {
	mangas, err := populate_mangas_with_frequency(userId)
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

	imageUrl, err := get_random_image_url(userId, mangaId)
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
