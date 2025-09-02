package main

import (
	"encoding/csv"
	"os"

	log "github.com/sirupsen/logrus"
)

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
				switch j {
				case 0:
					rec.MangaName = field
				case 1:
					rec.MangaId = field
				default:
					rec.ContentType = field
				}
			}
			MangaIds = append(MangaIds, rec)
		}
	}
}

func loadMangaCSV(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal("Error opening mangaIDs.csv", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1
	data, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Error parsing CSV", err)
	}
	fillMangas(data)
}
