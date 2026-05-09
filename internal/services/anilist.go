package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

const aniListGraphQLURL = "https://graphql.anilist.co"

const mangaQuery = `
query ($id: Int) {
  Media(id: $id, type: MANGA) {
    title { romaji english native }
    synonyms
    description(asHtml: false)
    coverImage { extraLarge large medium }
    bannerImage
    status
    format
    source
    countryOfOrigin
    genres
    tags { name rank }
    staff(sort: RELEVANCE, perPage: 25) {
      edges { role node { name { full } } }
    }
    averageScore
    siteUrl
    isAdult
    chapters
    volumes
    startDate { year month day }
    endDate { year month day }
  }
}
`

type aniListResponse struct {
	Data struct {
		Media *struct {
			Title struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
				Native  string `json:"native"`
			} `json:"title"`
			Synonyms    []string `json:"synonyms"`
			Description string   `json:"description"`
			CoverImage  struct {
				ExtraLarge string `json:"extraLarge"`
				Large      string `json:"large"`
				Medium     string `json:"medium"`
			} `json:"coverImage"`
			BannerImage     string   `json:"bannerImage"`
			Status          string   `json:"status"`
			Format          string   `json:"format"`
			Source          string   `json:"source"`
			CountryOfOrigin string   `json:"countryOfOrigin"`
			Genres          []string `json:"genres"`
			Tags            []struct {
				Name string `json:"name"`
				Rank *int   `json:"rank"`
			} `json:"tags"`
			Staff struct {
				Edges []struct {
					Role string `json:"role"`
					Node struct {
						Name struct {
							Full string `json:"full"`
						} `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"staff"`
			AverageScore *int   `json:"averageScore"`
			SiteURL      string `json:"siteUrl"`
			IsAdult      bool   `json:"isAdult"`
			Chapters     *int   `json:"chapters"`
			Volumes      *int   `json:"volumes"`
			StartDate    *struct {
				Year  *int `json:"year"`
				Month *int `json:"month"`
				Day   *int `json:"day"`
			} `json:"startDate"`
			EndDate *struct {
				Year  *int `json:"year"`
				Month *int `json:"month"`
				Day   *int `json:"day"`
			} `json:"endDate"`
		} `json:"Media"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func FetchAniListManga(ctx context.Context, id int) (*model.MangaPayload, error) {
	body, _ := json.Marshal(map[string]any{
		"query":     mangaQuery,
		"variables": map[string]int{"id": id},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aniListGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AniList API returned %d", resp.StatusCode)
	}

	var parsed aniListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Errors) > 0 {
		message := strings.TrimSpace(parsed.Errors[0].Message)
		if message == "" {
			message = "AniList API returned an unknown error"
		}
		return nil, fmt.Errorf("%s", message)
	}
	if parsed.Data.Media == nil {
		return nil, fmt.Errorf("manga not found on AniList")
	}

	media := parsed.Data.Media
	primary := firstNonEmpty(media.Title.English, media.Title.Romaji, media.Title.Native)
	alternatives := make([]string, 0)
	seen := map[string]struct{}{primary: {}}
	for _, title := range append([]string{media.Title.Romaji, media.Title.English, media.Title.Native}, media.Synonyms...) {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		alternatives = append(alternatives, title)
	}

	tagGroups := make([]model.TagGroup, 0, 2)
	if len(media.Genres) > 0 {
		tags := make([]model.Tag, 0, len(media.Genres))
		for _, genre := range media.Genres {
			tags = append(tags, model.Tag{ID: slugify(genre, "genre"), Title: genre})
		}
		tagGroups = append(tagGroups, model.TagGroup{ID: "genres", Title: "Genres", Tags: tags})
	}
	if len(media.Tags) > 0 {
		slices.SortFunc(media.Tags, func(a, b struct {
			Name string `json:"name"`
			Rank *int   `json:"rank"`
		}) int {
			ar, br := 0, 0
			if a.Rank != nil {
				ar = *a.Rank
			}
			if b.Rank != nil {
				br = *b.Rank
			}
			switch {
			case ar > br:
				return -1
			case ar < br:
				return 1
			default:
				return strings.Compare(a.Name, b.Name)
			}
		})
		tags := make([]model.Tag, 0, len(media.Tags))
		for _, tag := range media.Tags {
			tags = append(tags, model.Tag{ID: slugify(tag.Name, "tag"), Title: tag.Name})
		}
		tagGroups = append(tagGroups, model.TagGroup{ID: "tags", Title: "Tags", Tags: tags})
	}

	additionalInfo := make([]model.InfoEntry, 0)
	addInfo := func(key string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			additionalInfo = append(additionalInfo, model.InfoEntry{Key: key, Value: value})
		}
	}
	addInfo("Format", formatEnumLabel(media.Format))
	addInfo("Source", formatEnumLabel(media.Source))
	addInfo("Country of Origin", media.CountryOfOrigin)
	if media.Chapters != nil {
		addInfo("Chapters", fmt.Sprintf("%d", *media.Chapters))
	}
	if media.Volumes != nil {
		addInfo("Volumes", fmt.Sprintf("%d", *media.Volumes))
	}
	addInfo("Start Date", formatFuzzyDate(media.StartDate))
	addInfo("End Date", formatFuzzyDate(media.EndDate))

	var rating *float64
	if media.AverageScore != nil {
		value := float64(*media.AverageScore) / 10
		rating = &value
	}

	return &model.MangaPayload{
		PrimaryTitle:    primary,
		SecondaryTitles: alternatives,
		Synopsis:        stripHTML(media.Description),
		ThumbnailURL:    firstNonEmpty(media.CoverImage.ExtraLarge, media.CoverImage.Large, media.CoverImage.Medium),
		BannerURL:       strings.TrimSpace(media.BannerImage),
		ContentRating:   mapContentRating(media.IsAdult),
		Status:          mapStatus(media.Status),
		Artist:          findStaff(media.Staff.Edges, `\bart\b`),
		Author:          findStaff(media.Staff.Edges, `\bstory\b`),
		Rating:          rating,
		ShareURL:        strings.TrimSpace(media.SiteURL),
		ArtworkURLs:     compactUnique([]string{media.CoverImage.ExtraLarge, media.CoverImage.Large, media.CoverImage.Medium}),
		TagGroups:       tagGroups,
		AdditionalInfo:  additionalInfo,
	}, nil
}

func stripHTML(value string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	value = strings.ReplaceAll(value, "<br>", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	value = strings.ReplaceAll(value, "<br />", "\n")
	return strings.TrimSpace(re.ReplaceAllString(value, ""))
}

func formatEnumLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(value, "_", " ")), " ")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func formatFuzzyDate(date *struct {
	Year  *int `json:"year"`
	Month *int `json:"month"`
	Day   *int `json:"day"`
}) string {
	if date == nil || date.Year == nil {
		return ""
	}
	result := fmt.Sprintf("%d", *date.Year)
	if date.Month != nil {
		result += fmt.Sprintf("-%02d", *date.Month)
		if date.Day != nil {
			result += fmt.Sprintf("-%02d", *date.Day)
		}
	}
	return result
}

func mapStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "FINISHED":
		return "Completed"
	case "HIATUS":
		return "Hiatus"
	case "CANCELLED":
		return "Cancelled"
	default:
		return "Ongoing"
	}
}

func mapContentRating(isAdult bool) string {
	if isAdult {
		return "ADULT"
	}
	return "SAFE"
}

func findStaff(edges []struct {
	Role string `json:"role"`
	Node struct {
		Name struct {
			Full string `json:"full"`
		} `json:"name"`
	} `json:"node"`
}, pattern string) string {
	re := regexp.MustCompile(pattern)
	for _, edge := range edges {
		if re.MatchString(strings.ToLower(edge.Role)) {
			return strings.TrimSpace(edge.Node.Name.Full)
		}
	}
	return ""
}

func compactUnique(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func slugify(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	value = strings.Trim(re.ReplaceAllString(value, "-"), "-")
	if value == "" {
		return fallback
	}
	return value
}
