package main

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/gin-gonic/gin"
)

// This function is in charge of creating an array of strings that contain
// a manga each one,
func populateMangas() map[string]string {
	var mangaNames = map[string]string{}

	for i := 0; i < 4; i++ {
		var random int = rand.Intn(len(MangaIds))
		mangaNames[MangaIds[random].MangaName] = MangaIds[random].MangaId
	}

	for i, data := range mangaNames {
		fmt.Println(i, ":", data)
	}

	return mangaNames
}

func findMangas_sqlite() (gin.H, error) {

	return nil, errors.New("no mangaid matched")
}

func findMangas() (gin.H, error) {
	const maxRetries = 5

	for attempts := 0; attempts < maxRetries; attempts++ {
		// Key is the name of the manga, value is the ID.
		mangas := populateMangas()

		var mangaNames = []string{}

		for key := range mangas {
			mangaNames = append(mangaNames, key)
		}

		// Random number index that is going to be the manga image that we will display
		var correctNum int = rand.Intn(4)

		mangaId := mangas[mangaNames[correctNum]]
		fmt.Print(mangaId)
		if !(checkForVolumes(mangaId)) {
			continue
		}

		response := gin.H{
			"mangas":               mangaNames,
			"CurrentStoredMangaId": mangaId,
			"index":                correctNum,
		}

		attempts = maxRetries
		return response, nil
	}
	return nil, errors.New("no mangaid matched")
}
