// Package booksapi looks up book metadata by ISBN via the Google Books API.
package booksapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BookInfo struct {
	Title         string
	Author        string
	Publisher     string
	PublishedYear int
	CoverURL      string
}

var yearRe = regexp.MustCompile(`\b(1[89]\d\d|20\d\d)\b`)

var client = &http.Client{Timeout: 8 * time.Second}

func Lookup(ctx context.Context, isbn, apiKey string) (BookInfo, error) {
	isbn = strings.TrimSpace(strings.ReplaceAll(isbn, "-", ""))

	url := fmt.Sprintf(
		"https://www.googleapis.com/books/v1/volumes?q=isbn:%s",
		isbn,
	)
	if apiKey != "" {
		url += "&key=" + apiKey
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return BookInfo{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return BookInfo{}, fmt.Errorf("Google Books nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return BookInfo{}, fmt.Errorf("API-Anfragelimit erreicht, bitte später erneut versuchen")
	}
	if resp.StatusCode != 200 {
		return BookInfo{}, fmt.Errorf("unerwarteter Statuscode: %d", resp.StatusCode)
	}

	var raw struct {
		TotalItems int `json:"totalItems"`
		Items      []struct {
			VolumeInfo struct {
				Title         string   `json:"title"`
				Authors       []string `json:"authors"`
				Publisher     string   `json:"publisher"`
				PublishedDate string   `json:"publishedDate"`
				ImageLinks    struct {
					Thumbnail      string `json:"thumbnail"`
					SmallThumbnail string `json:"smallThumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return BookInfo{}, fmt.Errorf("ungültige Antwort: %w", err)
	}

	if raw.TotalItems == 0 || len(raw.Items) == 0 {
		return BookInfo{}, fmt.Errorf("ISBN %s nicht gefunden", isbn)
	}

	vi := raw.Items[0].VolumeInfo
	info := BookInfo{
		Title:     vi.Title,
		Publisher: vi.Publisher,
	}
	if len(vi.Authors) > 0 {
		info.Author = strings.Join(vi.Authors, ", ")
	}
	if m := yearRe.FindString(vi.PublishedDate); m != "" {
		info.PublishedYear, _ = strconv.Atoi(m)
	}
	// Google Books returns http URLs — upgrade to https
	cover := vi.ImageLinks.Thumbnail
	if cover == "" {
		cover = vi.ImageLinks.SmallThumbnail
	}
	info.CoverURL = strings.Replace(cover, "http://", "https://", 1)

	return info, nil
}
