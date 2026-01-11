package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

	_, err = DB.Exec("delete from user_manga_history where shown_at < datetime('now', '-7 days')")
	if err != nil {
		log.WithError(err).Error("Failed to cleanup old image history")
	}

	_, err = DB.Exec(`
		update user_manga_stats 
		set times_shown = 0, last_shown = datetime('now', '-30 days')
		where last_shown < datetime('now', '-30 days')
	`)
	if err != nil {
		log.WithError(err).Error("Failed to reset old manga stats")
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
	mangas, err := find_mangas_with_frequency(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No MangaId for some reason?"})
		return
	}

	gameState := GameState{
		MangaId:    mangas["CurrentStoredMangaId"].(string),
		CorrectNum: mangas["index"].(int),
		AtHome:     `https://api.mangadex.org/at-home/server/`,
	}

	if err := saveGameState(userId, gameState); err != nil {
		log.WithError(err).Error("Failed to save game state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save game state"})
		return
	}

	// Track manga selection in stats
	if err := updateMangaStats(userId, gameState.MangaId); err != nil {
		log.WithError(err).Error("Failed to update manga stats")
	}

	delete(mangas, "index")

	response := gin.H{"data": mangas}

	if token, err := GenerateJWT(userId); err == nil {
		response["token"] = token
	}

	c.JSON(http.StatusOK, response)
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
		response["token"] = token
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

	response := gin.H{}

	if token, err := GenerateJWT(userId); err == nil {
		response["token"] = token
	}

	if DB == nil {
		log.Error("Database not initialized")
		response["error"] = "Database not initialized"
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	var mangaName string
	err = DB.QueryRow("select name from mangas where id = ?", gameState.MangaId).Scan(&mangaName)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
			"error":   err,
		}).Error("Failed to get manga name")
		response["error"] = "Failed to get manga name"
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	rows, err := DB.Query("select image_name from images where manga_id = ?", gameState.MangaId)
	if err != nil {
		log.WithFields(log.Fields{
			"mangaId": gameState.MangaId,
			"error":   err,
		}).Error("Failed to query images")
		response["error"] = "Failed to query images"
		c.JSON(http.StatusInternalServerError, response)
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
		response["error"] = "No images found for manga"
		c.JSON(http.StatusNotFound, response)
		return
	}

	randomImageName := imageNames[rand.Intn(len(imageNames))]

	imageUrl := fmt.Sprintf("https://node1.alejoseed.com/mangas/%s/%s", mangaName, randomImageName)
	response["imageUrl"] = imageUrl

	c.JSON(http.StatusOK, response)
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

func updateMangaStats(userId string, mangaId string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	updateStatsQuery := `
		insert into user_manga_stats (user_id, manga_id, last_shown, times_shown)
		values (?, ?, datetime('now'), 1)
		on conflict(user_id, manga_id) 
		do update set 
			last_shown = datetime('now'),
			times_shown = times_shown + 1
	`

	_, err := DB.Exec(updateStatsQuery, userId, mangaId)
	if err != nil {
		return fmt.Errorf("failed to update manga stats: %w", err)
	}

	return nil
}

func trackImageViewed(userId string, mangaId string, imageName string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	updateHistoryQuery := `
		insert into user_manga_history (user_id, manga_id, shown_at, image_name, view_count)
		values (?, ?, datetime('now'), ?, 1)
		on conflict(user_id, manga_id, image_name) 
		do update set 
			view_count = view_count + 1,
			shown_at = datetime('now')
	`

	_, err := DB.Exec(updateHistoryQuery, userId, mangaId, imageName)
	if err != nil {
		return fmt.Errorf("failed to update image history: %w", err)
	}

	return nil
}

func getUserID(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := ValidateJWT(tokenString); err == nil {
			return claims.UserID, nil
		}
	}

	if tokenString := c.Query("token"); tokenString != "" {
		if claims, err := ValidateJWT(tokenString); err == nil {
			return claims.UserID, nil
		}
	}

	// Check JWT cookie (Old method getting rid of it)
	if tokenString, err := c.Cookie("mangaguesser_token"); err == nil && tokenString != "" {
		if claims, err := ValidateJWT(tokenString); err == nil {
			return claims.UserID, nil
		}
	}

	session := sessions.Default(c)
	if v := session.Get("userId"); v != nil {
		return v.(string), nil
	}

	return "", fmt.Errorf("no user ID found")
}
