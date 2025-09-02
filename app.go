package main

import (
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

var MangaIds = []manga{}
var GameStates = map[string]GameState{}

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Will log anything that is info or above (warn, error, fatal, panic).
	// Default.
	log.SetLevel(log.InfoLevel)

}

func main() {
	// load .env file
	err := godotenv.Load()

	if err != nil {
		log.Fatal("There was an error loading the .env file", err)
	}

	// Reading the file. It should be in the root
	loadMangaCSV("mangaIDs.csv")

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
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Cookie"}
	router.Use(cors.New(config))
	router.Use(sessions.Sessions("mysession", store))

	// Routes
	router.GET("/random_manga", randomManga)
	router.GET("/answer", checkAnswer)
	router.GET("/image", getImage)

	router.GET("/debug-session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"userId":    session.Get("userId"),
			"gameState": session.Get("gameState"),
		})
	})

	router.Run(":8080")
}
