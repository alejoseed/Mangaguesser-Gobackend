package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func randomManga(c *gin.Context) {
	session := sessions.Default(c)
	v := session.Get("userId")
	var userId string
	if v == nil {
		userId = generateUserId()
		session.Set("userId", userId)
	} else {
		userId = v.(string)
	}

	fmt.Println(userId)
	mangas, err := findMangas()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No MangaId for some reason?"})
		return
	}

	// Store the game state
	gameState := GameState{
		MangaId:    mangas["CurrentStoredMangaId"].(string),
		CorrectNum: mangas["index"].(int),
		AtHome:     `https://api.mangadex.org/at-home/server/`,
	}

	GameStates[userId] = gameState

	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}
	// Delete the obvious answer from the manga
	delete(mangas, "index")

	c.JSON(http.StatusOK, mangas)
}

func checkAnswer(c *gin.Context) {
	var userAnswerStr = c.Query("number")
	session := sessions.Default(c)
	var gameState GameState
	v := session.Get("userId")
	var userId string
	if v == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}
	userId = v.(string)
	gameState = GameStates[userId]

	userAnswer, err := strconv.Atoi(userAnswerStr)

	if err != nil {
		log.WithFields(log.Fields{
			"userAnswer": userAnswer,
			"err":        err,
		}).Error("There was an issue with the number formatting")

		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number format"})
		return
	}

	storedNumInterface := gameState.CorrectNum

	if userAnswer == storedNumInterface {
		c.JSON(http.StatusOK, gin.H{"correct": true})
	} else {
		log.Error("It got to this point so most likely there is a manga that it found?")
		c.JSON(http.StatusOK, gin.H{"correct": false})
	}
}

func getImage(c *gin.Context) {
	// For the sake of the test, we will just redo it, it is not necessary in prod
	//populateMangas()
	session := sessions.Default(c)
	var gameState GameState
	v := session.Get("userId")
	var userId string
	if v == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}

	userId = v.(string)
	gameState = GameStates[userId]

	requestUrl := "https://api.mangadex.org/manga/" + gameState.MangaId + "/aggregate"

	resp, err := http.Get(requestUrl)

	if err != nil {
		log.WithFields(log.Fields{
			"storedManga": gameState.MangaId,
			"url":         requestUrl,
			"error":       err,
		}).Error("Error reading response body in getImage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch url using gamestate mangaID."})
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"storedManga": gameState.MangaId,
			"url":         requestUrl,
			"status":      resp.StatusCode,
		}).Error("Unexpected status code from server in getImage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unexpected status code from server in getImage"})
		return
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.WithFields(log.Fields{
			"storedManga": gameState.MangaId,
			"url":         requestUrl,
			"error":       err,
		}).Error("Error reading response body in getImage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading response body in getImage"})
		return
	}

	var aggregate AggregateResponse
	err = json.Unmarshal(body, &aggregate)

	if err != nil {
		log.WithFields(log.Fields{
			"url": requestUrl,
			"err": err,
		}).Error("Failed to unmarshal aggregate response checkForVolumes")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal aggregate response checkForVolumes"})
		return
	}

	// Random selection logic
	volumeKeys := make([]string, 0, len(aggregate.Volumes))
	for k := range aggregate.Volumes {
		volumeKeys = append(volumeKeys, k)
	}

	randomVolumeKey := volumeKeys[rand.Intn(len(volumeKeys))]
	randomVolume := aggregate.Volumes[randomVolumeKey]

	chapterKeys := make([]string, 0, len(randomVolume.Chapters))
	for k := range randomVolume.Chapters {
		chapterKeys = append(chapterKeys, k)
	}

	randomChapterKey := chapterKeys[rand.Intn(len(chapterKeys))]
	randomChapter := randomVolume.Chapters[randomChapterKey]

	// We will hit this endpoint now that we have all the info
	var atHomeUrl strings.Builder

	atHomeUrl.WriteString("https://api.mangadex.org/at-home/server/")
	atHomeUrl.WriteString(randomChapter.ID)

	imageUrl := hitAtHomeUrl(atHomeUrl.String())

	// Stop if the imageUrl is an empty string, which means there is another
	// error logged with the issue.

	if len(imageUrl) == 0 {
		log.WithFields(log.Fields{
			"imageUrl": imageUrl,
		}).Error("There was an issue trying to get the URL for the image")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "There was an issue trying to get the URL for the image"})
		return
	}

	imageData, err := hitImageUrl(imageUrl)

	if err != nil {
		log.WithFields(log.Fields{
			"imageUrl": imageUrl,
			"err":      err,
		}).Error("Error fetching image data: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
		return
	}

	c.Header("Cache-Control", "max-age=0, no-cache, must-revalidate, proxy-revalidate")
	c.Data(http.StatusOK, "image/jpeg", imageData)
}
