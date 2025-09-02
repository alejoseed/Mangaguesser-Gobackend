package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
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
	requestUrl := "https://api.mangadex.org/manga/" + MangaId + "/aggregate"

	resp, err := http.Get(requestUrl)

	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": MangaId,
			"url":     requestUrl,
			"error":   err,
		}).Error("Failed to fetch manga volumes checkForVolumes")
		return false
	}
	defer resp.Body.Close()

	// Check the status code if request didn't fail
	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"mangaId": MangaId,
			"url":     requestUrl,
			"status":  resp.StatusCode,
		}).Error("Unexpected status code from server checkForVolumes")
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": MangaId,
			"url":     requestUrl,
			"error":   err,
		}).Error("Error reading response body checkForVolumes")
		return false
	}

	var aggregate AggregateResponse
	err = json.Unmarshal(body, &aggregate)

	if err != nil {
		log.WithFields(log.Fields{
			"url": requestUrl,
			"err": err,
		}).Error("Failed to unmarshal aggregate response checkForVolumes")
		return false
	}

	return len(aggregate.Volumes) > 0
}
