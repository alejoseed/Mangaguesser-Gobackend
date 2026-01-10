package main

import (
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func random_manga(c *gin.Context) {
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
	mangas, err := find_mangas()
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

func check_answer(c *gin.Context) {
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

func get_image(c *gin.Context) {
	session := sessions.Default(c)
	v := session.Get("userId")
	if v == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}
	userId := v.(string)
	gameState := GameStates[userId]

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./manga_images.db" // fallback to default
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to open database in get_image")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open database"})
		return
	}
	defer db.Close()

	var mangaName string
	err = db.QueryRow("select name from mangas where id = ?", gameState.MangaId).Scan(&mangaName)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
			"error":   err,
		}).Error("Failed to get manga name")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get manga name"})
		return
	}

	// Get all images for this manga
	rows, err := db.Query("select image_name from images where manga_id = ?", gameState.MangaId)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
			"error":   err,
		}).Error("Failed to query images")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query images"})
		return
	}
	defer rows.Close()

	var imageNames []string
	for rows.Next() {
		var imageName string
		if err := rows.Scan(&imageName); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to scan image name")
			continue
		}
		imageNames = append(imageNames, imageName)
	}

	if len(imageNames) == 0 {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
		}).Error("No images found for manga")
		c.JSON(http.StatusNotFound, gin.H{"error": "No images found for manga"})
		return
	}

	randomImageName := imageNames[rand.Intn(len(imageNames))]

	imageUrl := fmt.Sprintf("https://node1.alejoseed.com/mangas/%s/%s", mangaName, randomImageName)

	resp, err := http.Get(imageUrl)
	if err != nil {
		log.WithFields(log.Fields{
			"imageUrl": imageUrl,
			"error":    err,
		}).Error("Failed to fetch image from server")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"imageUrl": imageUrl,
			"status":   resp.StatusCode,
		}).Error("Unexpected status code from image server")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
		return
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"imageUrl": imageUrl,
			"error":    err,
		}).Error("Failed to read image data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read image"})
		return
	}

	c.Header("Cache-Control", "max-age=0, no-cache, must-revalidate, proxy-revalidate")
	c.Data(http.StatusOK, "image/png", imageData)
}
