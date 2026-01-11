package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func saveGameState(userId string, gameState GameState) error {
	gameStateJSON, err := json.Marshal(gameState)
	if err != nil {
		return fmt.Errorf("failed to marshal game state: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = DB.Exec(`
		insert or replace into sessions (user_id, game_state, expires_at) 
		values (?, ?, ?)
	`, userId, string(gameStateJSON), expiresAt)

	if err != nil {
		return fmt.Errorf("failed to save game state: %w", err)
	}

	return nil
}

func loadGameState(userId string) (GameState, error) {
	var gameStateJSON string
	var expiresAt time.Time

	err := DB.QueryRow(`
		select game_state, expires_at 
		from sessions 
		where user_id = ? AND expires_at > ?
	`, userId, time.Now()).Scan(&gameStateJSON, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return GameState{}, fmt.Errorf("no active session found")
		}
		return GameState{}, fmt.Errorf("failed to load game state: %w", err)
	}

	var gameState GameState
	err = json.Unmarshal([]byte(gameStateJSON), &gameState)
	if err != nil {
		return GameState{}, fmt.Errorf("failed to unmarshal game state: %w", err)
	}

	return gameState, nil
}

func cleanupExpiredSessions() {
	_, err := DB.Exec("delete from sessions where expires_at <= ?", time.Now())
	if err != nil {
		log.WithError(err).Error("Failed to cleanup expired sessions")
	}
}

func random_manga(c *gin.Context) {
	userId, err := getUserID(c)
	if err != nil {
		userId = generateUserId()
		session := sessions.Default(c)
		session.Set("userId", userId)
		if err := session.Save(); err != nil {
			log.WithError(err).Error("Failed to save session")
		}
	}

	fmt.Println(userId)
	mangas, err := find_mangas()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No MangaId for some reason?"})
		return
	}

	gameState := GameState{
		MangaId:    mangas["CurrentStoredMangaId"].(string),
		CorrectNum: mangas["index"].(int),
		AtHome:     `https://api.mangadex.org/at-home/server/`,
	}

	// Save to SQLite instead of memory map
	if err := saveGameState(userId, gameState); err != nil {
		log.WithError(err).Error("Failed to save game state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save game state"})
		return
	}

	delete(mangas, "index")

	token, err := GenerateJWT(userId)
	if err == nil {
		SetJWTCookie(c, token)
	}

	c.JSON(http.StatusOK, mangas)
}

func check_answer(c *gin.Context) {
	var userAnswerStr = c.Query("number")

	userId, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}

	gameState, err := loadGameState(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}

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

	correct := userAnswer == storedNumInterface
	response := gin.H{"correct": correct}

	if token, err := GenerateJWT(userId); err == nil {
		SetJWTCookie(c, token)
	}

	c.JSON(http.StatusOK, response)
}

func get_image(c *gin.Context) {
	userId, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}

	var gameState GameState
	gameState, err = loadGameState(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active game found"})
		return
	}

	if token, err := GenerateJWT(userId); err == nil {
		SetJWTCookie(c, token)
	}

	if DB == nil {
		log.Error("Database not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	var mangaName string
	err = DB.QueryRow("select name from mangas where id = ?", gameState.MangaId).Scan(&mangaName)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
			"error":   err,
		}).Error("Failed to get manga name")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get manga name"})
		return
	}

	rows, err := DB.Query("select image_name from images where manga_id = ?", gameState.MangaId)
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

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims, err := ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("userId", claims.UserID)
		c.Next()
	}
}

func getUserID(c *gin.Context) (string, error) {
	if tokenString, err := c.Cookie("mangaguesser_token"); err == nil && tokenString != "" {
		claims, err := ValidateJWT(tokenString)
		if err == nil {
			return claims.UserID, nil
		}
	}

	session := sessions.Default(c)
	if v := session.Get("userId"); v != nil {
		return v.(string), nil
	}

	return "", fmt.Errorf("no user ID found")
}
