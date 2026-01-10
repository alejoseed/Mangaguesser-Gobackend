package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

func generateUserId() string {
	id := uuid.New()
	return id.String()
}

func hitAtHomeUrl(atHomeParameters string) string {
	// Now we just need to get the imageUrl
	// First, we gotta hit the atHomeUrl, that has baseUrl + the image names
	// Including the dataSaver which is the one that we need.
	resp, err := http.Get(atHomeParameters)

	if err != nil {
		log.WithFields(log.Fields{
			"atHomeParameters": atHomeParameters,
			"error":            err,
		}).Error("Error fetching atHomeParameters at hitAtHomeUrl")
		return ""
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"atHomeParameters": atHomeParameters,
			"status":           resp.StatusCode,
		}).Error("Error fetching atHomeParameters at hitAtHomeUrl")
		return ""
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(log.Fields{
			"atHomeParameters": atHomeParameters,
			"error":            err,
		}).Error("Error reading response body in hitAtHomeUrl")
		return ""
	}

	var chapterImages ChapterImages

	err = json.Unmarshal(body, &chapterImages)

	if err != nil {
		log.WithFields(log.Fields{
			"atHomeParameters": atHomeParameters,
			"err":              err,
		}).Error("Failed to unmarshal aggregate response hitAtHomeUrl")
		return ""
	}

	// Let's make the imageUrl
	var imageUrl strings.Builder

	imageUrl.WriteString(chapterImages.BaseUrl)
	imageUrl.WriteString("/data-saver/")
	imageUrl.WriteString(chapterImages.Chapter.Hash)
	imageUrl.WriteString("/")
	middleOfArray := len(chapterImages.Chapter.DataSaver) / 2
	imageUrl.WriteString(chapterImages.Chapter.DataSaver[middleOfArray])

	return imageUrl.String()
}

func hitImageUrl(imageUrl string) ([]byte, error) {
	resp, err := http.Get(imageUrl)

	if err != nil {
		log.WithFields(log.Fields{
			"url":   imageUrl,
			"error": err,
		}).Error("Failed to make HTTP request for image at hitImageUrl")
		return nil, err
	}

	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		errorMessage := fmt.Sprintf("failed to fetch image: HTTP status code %d at hitImageUrl", resp.StatusCode)
		log.WithFields(log.Fields{
			"url":    imageUrl,
			"status": resp.StatusCode,
		}).Error(errorMessage)
		return nil, fmt.Errorf(errorMessage)
	}

	// Read the body to get the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"url":   imageUrl,
			"error": err,
		}).Error("Failed to read response body for image at hitImageUrl")
		return nil, err
	}

	return data, nil
}

func checkForVolumes(MangaId string) bool {
	// Create database connection
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./manga_images.db" // fallback to default
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": MangaId,
			"error":   err,
		}).Error("Failed to open database checkForVolumes")
		return false
	}
	defer db.Close()

	// Check if this manga has any images in our local database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM images WHERE manga_id = ?", MangaId).Scan(&count)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": MangaId,
			"error":   err,
		}).Error("Failed to query images checkForVolumes")
		return false
	}

	return count > 0
}
