package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type manga struct {
	ContentType string `json:"contentType"`
	MangaId     string `json:"MangaId"`
	MangaName   string `json:"mangaName"`
}

type AggregateResponse struct {
	Volumes map[string]Volume `json:"volumes"`
}

type Volume struct {
	Chapters map[string]Chapter `json:"chapters"`
}

type Chapter struct {
	ID string `json:"id"`
}

type ChapterImages struct {
	Result  string        `json:"result"`
	BaseUrl string        `json:"baseUrl"`
	Chapter ChapterDetail `json:"chapter"`
}

type ChapterDetail struct {
	Hash      string   `json:"hash"`
	Data      []string `json:"data"`
	DataSaver []string `json:"dataSaver"`
}

var MangaIds = []manga{}

/*
The function is obvious. It gets the data from the CSV
and fills it into an array of manga that will be properly
formatted based on the field.. It returns void because
the manga has to be global since it will be accessed
via an endpoint
*/
func fillMangas(data [][]string) {
	for i, line := range data {
		if i > 0 {
			var rec manga
			for j, field := range line {
				if j == 0 {
					rec.MangaName = field
				} else if j == 1 {
					rec.MangaId = field
				} else {
					rec.ContentType = field
				}
			}
			MangaIds = append(MangaIds, rec)
		}
	}
}

// This function is in charge of creating an array of strings that contain
// a manga each one,
func populateMangas() map[string]string {

	// // Get a correct num, this will be the one we pass
	// // in the future to check the answer
	// var correctNum int = rand.Intn(4)
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

func randomManga(c *gin.Context) {

	session := sessions.Default(c)

	const maxRetries = 5

	for attempts := 0; attempts < maxRetries; attempts++ {
		mangas := populateMangas()
		var mangasArray = []string{}

		for key := range mangas {
			mangasArray = append(mangasArray, key)
		}

		// Random number that is going to be the manga image that we will display
		var correctNum int = rand.Intn(4)

		// Add the correct number to the session token..
		session.Set("correctNum", correctNum)
		session.Set("MangaId", mangas[mangasArray[correctNum]])
		session.Set("atHome", "https://api.mangadex.org/at-home/server/")
		session.Save()

		// If the manga has as volume that means there are images around to get
		if storedMangaId, ok := session.Get("MangaId").(string); ok {
			if checkForVolumes(storedMangaId) {
				response := gin.H{
					"mangas":               mangasArray,
					"CurrentStoredMangaId": storedMangaId,
				}

				c.IndentedJSON(http.StatusOK, response)
				break
			} else {
				// No volume for that manga so let's continue the for loop
				continue
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No MangaId for some reason?"})
		}

	}
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

func checkAnswer(c *gin.Context) {
	var userAnswerStr = c.Query("number")
	session := sessions.Default(c)
	userAnswer, err := strconv.Atoi(userAnswerStr)

	if err != nil {
		log.WithFields(log.Fields{
			"userAnswer": userAnswer,
			"err":        err,
		}).Error("There was an issue with the number formatting")

		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number format"})
		return
	}

	storedNumInterface := session.Get("correctNum")

	if storedNum, ok := storedNumInterface.(int); ok {
		isCorrect := (userAnswer == storedNum)
		c.JSON(http.StatusOK, gin.H{"correct": isCorrect})
	} else {
		log.Error("Session data corrupted or missing, no random_manga?")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session data corrupted or missing, no random_manga?"})
	}
}

func getImage(c *gin.Context) {

	// For the sake of the test, we will just redo it, it is not necessary in prod
	//populateMangas()
	session := sessions.Default(c)
	MangaIdInterface := session.Get("MangaId")

	if storedManga, ok := MangaIdInterface.(string); ok {
		requestUrl := "https://api.mangadex.org/manga/" + storedManga + "/aggregate"

		resp, err := http.Get(requestUrl)

		if err != nil {
			log.WithFields(log.Fields{
				"storedManga": storedManga,
				"url":         requestUrl,
				"error":       err,
			}).Error("Error reading response body in getImage")
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.WithFields(log.Fields{
				"storedManga": storedManga,
				"url":         requestUrl,
				"status":      resp.StatusCode,
			}).Error("Unexpected status code from server in getImage")
			return
		}

		body, err := io.ReadAll(resp.Body)

		if err != nil {
			log.WithFields(log.Fields{
				"storedManga": storedManga,
				"url":         requestUrl,
				"error":       err,
			}).Error("Error reading response body in getImage")
			return
		}

		var aggregate AggregateResponse
		err = json.Unmarshal(body, &aggregate)

		if err != nil {
			log.WithFields(log.Fields{
				"url": requestUrl,
				"err": err,
			}).Error("Failed to unmarshal aggregate response checkForVolumes")
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
			return
		}

		imageData, err := hitImageUrl(imageUrl)

		if err != nil {
			log.WithFields(log.Fields{
				"imageUrl": imageUrl,
				"err":      err,
			}).Error("Error fetching image data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
			return
		}
		c.Data(http.StatusOK, "image/jpeg", imageData)

	} else {
		log.Error("error", "Issue on storedManga")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Issue on storedManga"})
	}
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
	file, err := os.Open("mangaIDs.csv")

	if err != nil {
		log.Error("There was an error opening mangaIDs.csv", err)
	}

	defer file.Close()

	// Disable the checking in case there is
	// an extra/missing comma.
	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1

	data, err := csvReader.ReadAll()

	if err != nil {
		log.Error("There was an parsing the body of the CSV", err)
	}

	// Fill in the manga array with the data from the CSV
	fillMangas(data)

	sessionSecret := os.Getenv("SESSION_SECRET")

	router := gin.Default()

	// Session middleware
	store := cookie.NewStore([]byte(sessionSecret))
	router.Use(sessions.Sessions("mysession", store))
	router.Use(cors.Default())
	// Routes
	router.GET("/random_manga", randomManga)
	router.GET("/answer", checkAnswer)
	router.GET("/image", getImage)
	router.Run(":8080")

}
