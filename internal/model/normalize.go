package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var discoverSectionTypes = map[string]struct{}{
	"featured":          {},
	"simpleCarousel":    {},
	"prominentCarousel": {},
	"chapterUpdates":    {},
	"genres":            {},
}

var discoverSectionModes = map[string]struct{}{
	"manual": {},
	"live":   {},
}

var discoverItemTypes = map[string]struct{}{
	"featuredCarouselItem":       {},
	"simpleCarouselItem":         {},
	"prominentCarouselItem":      {},
	"chapterUpdatesCarouselItem": {},
	"genresCarouselItem":         {},
}

var contentRatings = map[string]struct{}{
	"SAFE":   {},
	"MATURE": {},
	"ADULT":  {},
}

func NormalizeMangaPayload(payload MangaPayload) MangaPayload {
	payload.PrimaryTitle = strings.TrimSpace(payload.PrimaryTitle)
	payload.Synopsis = strings.TrimSpace(payload.Synopsis)
	payload.ThumbnailURL = strings.TrimSpace(payload.ThumbnailURL)
	payload.BannerURL = strings.TrimSpace(payload.BannerURL)
	payload.ContentRating = defaultString(payload.ContentRating, "SAFE")
	payload.Status = defaultString(payload.Status, "Ongoing")
	payload.Artist = strings.TrimSpace(payload.Artist)
	payload.Author = strings.TrimSpace(payload.Author)
	payload.ShareURL = strings.TrimSpace(payload.ShareURL)
	payload.SecondaryTitles = NormalizeStringList(payload.SecondaryTitles)
	payload.ArtworkURLs = NormalizeStringList(payload.ArtworkURLs)
	payload.AdditionalInfo = NormalizeInfoEntries(payload.AdditionalInfo)
	payload.TagGroups = NormalizeTagGroups(payload.TagGroups)
	return payload
}

func NormalizeChapterPayload(payload ChapterPayload) ChapterPayload {
	payload.LangCode = defaultString(payload.LangCode, "EN")
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Pages = NormalizeStringList(payload.Pages)
	payload.AdditionalInfo = NormalizeInfoEntries(payload.AdditionalInfo)
	return payload
}

func NormalizeChapterImportPayload(payload ChapterImportPayload) ChapterImportPayload {
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.Mode = strings.ToLower(strings.TrimSpace(payload.Mode))
	payload.Config = normalizeRawJSON(payload.Config)
	return payload
}

func NormalizeDiscoverPayload(payload DiscoverSectionPayload) DiscoverSectionPayload {
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Subtitle = strings.TrimSpace(payload.Subtitle)
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Mode = defaultString(strings.TrimSpace(payload.Mode), "manual")
	if payload.LiveRule != nil {
		payload.LiveRule.Preset = strings.TrimSpace(payload.LiveRule.Preset)
	}
	items := make([]DiscoverSectionItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		item.ID = strings.TrimSpace(item.ID)
		item.Type = strings.TrimSpace(item.Type)
		item.MangaID = strings.TrimSpace(item.MangaID)
		item.ChapterID = strings.TrimSpace(item.ChapterID)
		item.ImageURL = strings.TrimSpace(item.ImageURL)
		item.Title = strings.TrimSpace(item.Title)
		item.Subtitle = strings.TrimSpace(item.Subtitle)
		item.Supertitle = strings.TrimSpace(item.Supertitle)
		item.Name = strings.TrimSpace(item.Name)
		if item.ContentRating != nil {
			value := strings.TrimSpace(*item.ContentRating)
			item.ContentRating = &value
		}
		item.Metadata = normalizeRawJSON(item.Metadata)
		if item.SearchQuery != nil {
			item.SearchQuery.Title = strings.TrimSpace(item.SearchQuery.Title)
			seenFilters := make(map[string]struct{}, len(item.SearchQuery.Filters))
			filters := make([]SearchFilter, 0, len(item.SearchQuery.Filters))
			for _, filter := range item.SearchQuery.Filters {
				filter.ID = strings.TrimSpace(filter.ID)
				filter.Value = normalizeFilterValue(filter.Value)
				if filter.ID != "" {
					if _, ok := seenFilters[filter.ID]; ok {
						continue
					}
					seenFilters[filter.ID] = struct{}{}
					filters = append(filters, filter)
				}
			}
			item.SearchQuery.Filters = filters
		}
		if isValidDiscoverItemType(item.Type) {
			item = normalizeDiscoverItemForType(item)
		}
		items = append(items, item)
	}
	if payload.Mode == "live" {
		payload.Items = []DiscoverSectionItem{}
	} else {
		payload.Items = items
	}
	return payload
}

func ValidateDiscoverPayload(payload DiscoverSectionPayload) error {
	if payload.Title == "" {
		return fmt.Errorf("title is required")
	}
	if !isValidDiscoverSectionType(payload.Type) {
		return fmt.Errorf("invalid discover section type %q", payload.Type)
	}
	if !isValidDiscoverSectionMode(payload.Mode) {
		return fmt.Errorf("invalid discover section mode %q", payload.Mode)
	}
	if payload.Mode == "manual" && len(payload.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}
	if payload.Mode == "live" {
		if payload.LiveRule == nil {
			return fmt.Errorf("live sections require liveRule")
		}
		if payload.LiveRule.Limit <= 0 {
			return fmt.Errorf("live sections require liveRule.limit to be greater than zero")
		}
		if !isValidDiscoverLivePreset(payload.Type, payload.LiveRule.Preset) {
			return fmt.Errorf("invalid liveRule preset %q for section %q", payload.LiveRule.Preset, payload.Type)
		}
		return nil
	}

	expectedItemType := expectedDiscoverItemType(payload.Type)
	for index, item := range payload.Items {
		label := fmt.Sprintf("item %d", index+1)
		if item.Type != expectedItemType {
			return fmt.Errorf("%s must use type %q for section %q", label, expectedItemType, payload.Type)
		}
		if !isValidDiscoverItemType(item.Type) {
			return fmt.Errorf("%s has invalid type %q", label, item.Type)
		}
		if item.ContentRating != nil && !isValidContentRating(*item.ContentRating) {
			return fmt.Errorf("%s has invalid content rating %q", label, *item.ContentRating)
		}
		if len(item.Metadata) > 0 && !json.Valid(item.Metadata) {
			return fmt.Errorf("%s has invalid metadata JSON", label)
		}
		if err := validateSearchQuery(label, item.SearchQuery); err != nil {
			return err
		}

		switch item.Type {
		case "featuredCarouselItem", "simpleCarouselItem", "prominentCarouselItem":
			if item.MangaID == "" {
				return fmt.Errorf("%s requires mangaId", label)
			}
			if item.ImageURL == "" {
				return fmt.Errorf("%s requires imageUrl", label)
			}
			if item.Title == "" {
				return fmt.Errorf("%s requires title", label)
			}
		case "chapterUpdatesCarouselItem":
			if item.MangaID == "" {
				return fmt.Errorf("%s requires mangaId", label)
			}
			if item.ChapterID == "" {
				return fmt.Errorf("%s requires chapterId", label)
			}
			if item.ImageURL == "" {
				return fmt.Errorf("%s requires imageUrl", label)
			}
			if item.Title == "" {
				return fmt.Errorf("%s requires title", label)
			}
		case "genresCarouselItem":
			if item.Name == "" {
				return fmt.Errorf("%s requires name", label)
			}
			if item.SearchQuery == nil {
				return fmt.Errorf("%s requires searchQuery", label)
			}
			if item.SearchQuery.Title == "" {
				return fmt.Errorf("%s requires searchQuery.title", label)
			}
		}
	}

	return nil
}

func NormalizeInfoEntries(entries []InfoEntry) []InfoEntry {
	result := make([]InfoEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Key = strings.TrimSpace(entry.Key)
		entry.Value = strings.TrimSpace(entry.Value)
		if entry.Key != "" {
			result = append(result, entry)
		}
	}
	return result
}

func NormalizeStringList(values []string) []string {
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

func NormalizeTagGroups(groups []TagGroup) []TagGroup {
	result := make([]TagGroup, 0, len(groups))
	for _, group := range groups {
		group.ID = strings.TrimSpace(group.ID)
		group.Title = strings.TrimSpace(group.Title)
		if group.Title == "" {
			continue
		}
		tags := make([]Tag, 0, len(group.Tags))
		for _, tag := range group.Tags {
			tag.ID = strings.TrimSpace(tag.ID)
			tag.Title = strings.TrimSpace(tag.Title)
			if tag.Title != "" {
				tags = append(tags, tag)
			}
		}
		group.Tags = tags
		result = append(result, group)
	}
	return result
}

func ParseISOTimePointer(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}

	if unix, err := time.Parse(time.RFC3339, trimmed); err == nil {
		utc := unix.UTC()
		return &utc, nil
	}
	if day, err := time.Parse("2006-01-02", trimmed); err == nil {
		utc := day.UTC()
		return &utc, nil
	}
	return nil, fmt.Errorf("invalid time value: %s", trimmed)
}

func MustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func expectedDiscoverItemType(sectionType string) string {
	switch strings.TrimSpace(sectionType) {
	case "featured":
		return "featuredCarouselItem"
	case "prominentCarousel":
		return "prominentCarouselItem"
	case "chapterUpdates":
		return "chapterUpdatesCarouselItem"
	case "genres":
		return "genresCarouselItem"
	default:
		return "simpleCarouselItem"
	}
}

func normalizeDiscoverItemForType(item DiscoverSectionItem) DiscoverSectionItem {
	switch item.Type {
	case "featuredCarouselItem":
		item.ChapterID = ""
		item.Subtitle = ""
		item.Name = ""
		item.PublishDate = nil
		item.SearchQuery = nil
	case "simpleCarouselItem", "prominentCarouselItem":
		item.ChapterID = ""
		item.Supertitle = ""
		item.Name = ""
		item.PublishDate = nil
		item.SearchQuery = nil
	case "chapterUpdatesCarouselItem":
		item.Supertitle = ""
		item.Name = ""
		item.SearchQuery = nil
	case "genresCarouselItem":
		item.MangaID = ""
		item.ChapterID = ""
		item.ImageURL = ""
		item.Title = ""
		item.Subtitle = ""
		item.Supertitle = ""
		item.PublishDate = nil
	}
	return item
}

func normalizeRawJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, value); err != nil {
		return json.RawMessage(bytes.TrimSpace(value))
	}
	return json.RawMessage(buf.Bytes())
}

func normalizeFilterValue(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return normalizeRawJSON(value)
}

func validateSearchQuery(label string, query *DiscoverSearchQuery) error {
	if query == nil {
		return nil
	}
	if query.Title == "" {
		return fmt.Errorf("%s requires a non-empty searchQuery.title", label)
	}
	for _, filter := range query.Filters {
		if filter.ID == "" {
			return fmt.Errorf("%s includes a search filter with an empty id", label)
		}
		if len(filter.Value) == 0 || !json.Valid(filter.Value) {
			return fmt.Errorf("%s includes invalid JSON for search filter %q", label, filter.ID)
		}
	}
	return nil
}

func isValidDiscoverSectionType(value string) bool {
	_, ok := discoverSectionTypes[value]
	return ok
}

func isValidDiscoverSectionMode(value string) bool {
	_, ok := discoverSectionModes[value]
	return ok
}

func isValidDiscoverItemType(value string) bool {
	_, ok := discoverItemTypes[value]
	return ok
}

func isValidContentRating(value string) bool {
	_, ok := contentRatings[strings.TrimSpace(value)]
	return ok
}

func isValidDiscoverLivePreset(sectionType string, preset string) bool {
	switch strings.TrimSpace(sectionType) {
	case "featured", "simpleCarousel", "prominentCarousel":
		switch strings.TrimSpace(preset) {
		case "title_asc", "title_desc", "rating_desc", "updated_desc", "chapters_desc":
			return true
		}
	case "chapterUpdates":
		return strings.TrimSpace(preset) == "latest_chapters"
	case "genres":
		switch strings.TrimSpace(preset) {
		case "genres_az", "genres_za", "genres_top":
			return true
		}
	}
	return false
}
