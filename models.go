package main

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

type GameState struct {
	MangaId    string `json:"mangaId"`
	CorrectNum int    `json:"correctNum"`
	AtHome     string `json:"atHome"`
}
