package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

var DB *sql.DB

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func initDB() error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./manga_images.db"
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("There was an error loading the .env file", err)
	}

	// Initialize database
	err = initDB()
	if err != nil {
		log.Fatal("Failed to initialize database", err)
	}

	sessionSecret := os.Getenv("SESSION_SECRET")

	router := gin.New()
	router.Use(gin.Recovery())

	router.Use(func(c *gin.Context) {
		c.Next()

		status := c.Writer.Status()
		if status == http.StatusOK || status == http.StatusBadRequest || status == http.StatusInternalServerError {
			log.Printf("%d | %s | %s | %s", status, c.Request.Method, c.Request.URL.Path, c.ClientIP())
		}
	})

	// Session middleware
	store := cookie.NewStore([]byte(sessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowOrigins = []string{"http://localhost:8080", "http://localhost:3000", "https://mangaguesser.alejoseed.com"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Cookie", "Authorization"}
	router.Use(cors.New(config))
	router.Use(sessions.Sessions("mysession", store))

	// Game Routes (work with both HTTP-only session and client-side JWT cookie)
	router.GET("/random_manga", random_manga)
	router.GET("/answer", check_answer)
	router.GET("/image", get_image)

	router.GET("/debug-session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"userId":    session.Get("userId"),
			"gameState": session.Get("gameState"),
		})
	})

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cleanupExpiredSessions()
			}
		}
	}()

	router.Run(":8080")
}
