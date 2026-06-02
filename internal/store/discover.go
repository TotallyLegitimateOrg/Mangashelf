package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func (s *Store) ListDiscoverSections(ctx context.Context) ([]model.DiscoverSection, error) {
	configs, err := s.ListDiscoverSectionConfigs(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]model.DiscoverSection, 0, len(configs))
	for _, section := range configs {
		result = append(result, model.DiscoverSection{
			ID:        section.ID,
			Title:     section.Title,
			Subtitle:  section.Subtitle,
			Type:      section.Type,
			SortOrder: section.SortOrder,
			Items:     section.Items,
		})
	}
	return result, nil
}

func (s *Store) ListDiscoverSectionConfigs(ctx context.Context) ([]model.DiscoverSectionConfig, error) {
	configs, err := s.listStoredDiscoverSectionConfigs(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]model.DiscoverSectionConfig, 0, len(configs))
	for _, section := range configs {
		if section.Mode == "live" && section.LiveRule != nil {
			liveItems, err := s.liveDiscoverItems(ctx, section.Type, *section.LiveRule)
			if err != nil {
				return nil, err
			}
			section.Items = liveItems
		}
		result = append(result, section)
	}
	return result, nil
}

func (s *Store) listStoredDiscoverSectionConfigs(ctx context.Context) ([]model.DiscoverSectionConfig, error) {
	sections, err := s.queries.ListDiscoverSections(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.queries.ListDiscoverItems(ctx)
	if err != nil {
		return nil, err
	}

	itemsBySection := make(map[string][]model.DiscoverSectionItem, len(sections))
	for _, item := range items {
		converted := model.DiscoverSectionItem{
			ID:            item.ID,
			Type:          item.ItemType,
			MangaID:       item.MangaID.String,
			ChapterID:     item.ChapterID.String,
			ChapNum:       nil,
			ImageURL:      item.ImageUrl,
			Title:         item.Title,
			Subtitle:      item.Subtitle,
			Supertitle:    item.Supertitle,
			Name:          item.Name,
			PublishDate:   isoStringPtrFromNull(item.PublishDate),
			ContentRating: nil,
			Metadata:      nil,
			SearchQuery:   nil,
		}
		if item.ContentRating.Valid {
			value := item.ContentRating.String
			converted.ContentRating = &value
		}
		if item.MetadataJson.Valid {
			metadata := json.RawMessage(item.MetadataJson.String)
			if !json.Valid(metadata) {
				return nil, fmt.Errorf("discover item %s has invalid metadata JSON", item.ID)
			}
			converted.Metadata = metadata
		}
		if item.SearchQueryJson.Valid {
			var query model.DiscoverSearchQuery
			if err := json.Unmarshal([]byte(item.SearchQueryJson.String), &query); err != nil {
				return nil, fmt.Errorf("discover item %s has invalid search query JSON: %w", item.ID, err)
			}
			converted.SearchQuery = &query
		}
		itemsBySection[item.SectionID] = append(itemsBySection[item.SectionID], converted)
	}

	result := make([]model.DiscoverSectionConfig, 0, len(sections))
	for _, section := range sections {
		var liveRule *model.DiscoverLiveRule
		if section.LiveRuleJson.Valid {
			var parsed model.DiscoverLiveRule
			if err := json.Unmarshal([]byte(section.LiveRuleJson.String), &parsed); err != nil {
				return nil, fmt.Errorf("discover section %s has invalid live rule JSON: %w", section.ID, err)
			}
			liveRule = &parsed
		}
		sectionItems := itemsBySection[section.ID]
		if sectionItems == nil {
			sectionItems = []model.DiscoverSectionItem{}
		}
		result = append(result, model.DiscoverSectionConfig{
			ID:        section.ID,
			Title:     section.Title,
			Subtitle:  section.Subtitle,
			Type:      section.SectionType,
			Mode:      section.Mode,
			LiveRule:  liveRule,
			SortOrder: int(section.SortOrder),
			Items:     sectionItems,
		})
	}
	return result, nil
}

func (s *Store) CreateDiscoverSection(ctx context.Context, payload model.DiscoverSectionPayload) (*model.DiscoverSectionConfig, error) {
	payload = model.NormalizeDiscoverPayload(payload)
	if err := model.ValidateDiscoverPayload(payload); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(q *gen.Queries) error {
		maxSortOrder, err := q.GetMaxDiscoverSortOrder(ctx)
		if err != nil {
			return err
		}
		if err := q.CreateDiscoverSection(ctx, gen.CreateDiscoverSectionParams{
			ID:           id,
			Title:        payload.Title,
			Subtitle:     payload.Subtitle,
			SectionType:  payload.Type,
			SortOrder:    maxSortOrder + 1,
			Mode:         payload.Mode,
			LiveRuleJson: discoverLiveRuleValue(payload.LiveRule),
		}); err != nil {
			return err
		}
		return replaceDiscoverItems(ctx, q, id, payload.Items)
	})
	if err != nil {
		return nil, err
	}
	return s.findDiscoverSection(ctx, id)
}

func (s *Store) UpdateDiscoverSection(ctx context.Context, id string, payload model.DiscoverSectionPayload) (*model.DiscoverSectionConfig, error) {
	payload = model.NormalizeDiscoverPayload(payload)
	if err := model.ValidateDiscoverPayload(payload); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	err := s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.UpdateDiscoverSection(ctx, gen.UpdateDiscoverSectionParams{
			Title:        payload.Title,
			Subtitle:     payload.Subtitle,
			SectionType:  payload.Type,
			Mode:         payload.Mode,
			LiveRuleJson: discoverLiveRuleValue(payload.LiveRule),
			ID:           id,
		}); err != nil {
			return err
		}
		return replaceDiscoverItems(ctx, q, id, payload.Items)
	})
	if err != nil {
		return nil, err
	}
	return s.findDiscoverSection(ctx, id)
}

func (s *Store) DeleteDiscoverSection(ctx context.Context, id string) error {
	return s.queries.DeleteDiscoverSection(ctx, id)
}

func (s *Store) ReorderDiscoverSections(ctx context.Context, order []string) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		for index, id := range order {
			if err := q.UpdateDiscoverSectionSortOrder(ctx, gen.UpdateDiscoverSectionSortOrderParams{
				SortOrder: int64(index),
				ID:        id,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) findDiscoverSection(ctx context.Context, id string) (*model.DiscoverSectionConfig, error) {
	sections, err := s.ListDiscoverSectionConfigs(ctx)
	if err != nil {
		return nil, err
	}
	for _, section := range sections {
		if section.ID == id {
			return &section, nil
		}
	}
	return nil, ErrNotFound
}

func replaceDiscoverItems(ctx context.Context, q *gen.Queries, sectionID string, items []model.DiscoverSectionItem) error {
	if err := q.DeleteDiscoverItemsBySectionID(ctx, sectionID); err != nil {
		return err
	}
	for index, item := range items {
		itemID := item.ID
		if itemID == "" {
			var err error
			itemID, err = newID()
			if err != nil {
				return err
			}
		} else if !isUUIDv7(itemID) {
			return fmt.Errorf("%w: discover item %s id must be UUIDv7", ErrValidation, itemID)
		}
		var metadata sql.NullString
		if item.Metadata != nil {
			data, _ := json.Marshal(item.Metadata)
			metadata = sql.NullString{String: string(data), Valid: true}
		}
		var searchQuery sql.NullString
		if item.SearchQuery != nil {
			data, _ := json.Marshal(item.SearchQuery)
			searchQuery = sql.NullString{String: string(data), Valid: true}
		}
		if err := q.InsertDiscoverItem(ctx, gen.InsertDiscoverItemParams{
			ID:              itemID,
			SectionID:       sectionID,
			ItemType:        item.Type,
			SortOrder:       int64(index),
			MangaID:         nullString(item.MangaID),
			ChapterID:       nullString(item.ChapterID),
			ImageUrl:        item.ImageURL,
			Title:           item.Title,
			Subtitle:        item.Subtitle,
			Supertitle:      item.Supertitle,
			Name:            item.Name,
			PublishDate:     discoverPublishDate(item.PublishDate),
			ContentRating:   nullableStringPtr(item.ContentRating),
			MetadataJson:    metadata,
			SearchQueryJson: searchQuery,
		}); err != nil {
			return err
		}
	}
	return nil
}

func discoverPublishDate(value *string) sql.NullInt64 {
	parsed, err := parseTimePointer(value)
	if err != nil || parsed == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: parsed.Unix(), Valid: true}
}

func nullableStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullString(*value)
}

func discoverLiveRuleValue(rule *model.DiscoverLiveRule) sql.NullString {
	if rule == nil {
		return sql.NullString{}
	}
	data, _ := json.Marshal(rule)
	return sql.NullString{String: string(data), Valid: true}
}

func (s *Store) liveDiscoverItems(ctx context.Context, sectionType string, rule model.DiscoverLiveRule) ([]model.DiscoverSectionItem, error) {
	switch sectionType {
	case "featured", "simpleCarousel", "prominentCarousel":
		return s.liveMangaDiscoverItems(ctx, sectionType, rule)
	case "chapterUpdates":
		return s.liveChapterUpdateItems(ctx, rule.Limit)
	case "genres":
		return s.liveGenreItems(ctx, rule)
	default:
		return []model.DiscoverSectionItem{}, nil
	}
}

func (s *Store) liveMangaDiscoverItems(ctx context.Context, sectionType string, rule model.DiscoverLiveRule) ([]model.DiscoverSectionItem, error) {
	manga, err := s.SearchManga(ctx, model.MangaSearchOptions{Sort: rule.Preset})
	if err != nil {
		return nil, err
	}
	if len(manga) > rule.Limit {
		manga = manga[:rule.Limit]
	}

	items := make([]model.DiscoverSectionItem, 0, len(manga))
	for _, entry := range manga {
		item := model.DiscoverSectionItem{
			ID:            liveDiscoverItemID(sectionType, entry.ID),
			MangaID:       entry.ID,
			ImageURL:      entry.ThumbnailURL,
			Title:         entry.PrimaryTitle,
			ContentRating: stringPointerOrNil(entry.ContentRating),
		}
		switch sectionType {
		case "featured":
			item.Type = "featuredCarouselItem"
		case "prominentCarousel":
			item.Type = "prominentCarouselItem"
			item.Subtitle = discoverMangaSubtitle(entry)
		default:
			item.Type = "simpleCarouselItem"
			item.Subtitle = discoverMangaSubtitle(entry)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) liveChapterUpdateItems(ctx context.Context, limit int) ([]model.DiscoverSectionItem, error) {
	rows, err := s.queries.ListRecentStoredChapters(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	items := make([]model.DiscoverSectionItem, 0, len(rows))
	for _, row := range rows {
		chapNum := row.ChapNum
		items = append(items, model.DiscoverSectionItem{
			ID:          liveDiscoverItemID("chapterUpdates", row.ID),
			Type:        "chapterUpdatesCarouselItem",
			MangaID:     row.MangaID,
			ChapterID:   row.ID,
			ChapNum:     &chapNum,
			ImageURL:    row.ThumbnailUrl,
			Title:       row.PrimaryTitle,
			Subtitle:    discoverChapterSubtitle(row.Title, row.ChapNum),
			PublishDate: isoStringPtrFromNull(row.PublishDate),
		})
	}
	return items, nil
}

func (s *Store) liveGenreItems(ctx context.Context, rule model.DiscoverLiveRule) ([]model.DiscoverSectionItem, error) {
	rows, err := s.queries.ListGenreTagCounts(ctx)
	if err != nil {
		return nil, err
	}

	type genreEntry struct {
		label      string
		queryValue string
		mangaCount int64
	}

	entries := make([]genreEntry, 0, len(rows))
	for _, row := range rows {
		label := row.TagTitle
		if !strings.EqualFold(strings.TrimSpace(row.GroupTitle), "Genres") {
			label = fmt.Sprintf("%s: %s", row.GroupTitle, row.TagTitle)
		}
		entries = append(entries, genreEntry{
			label:      label,
			queryValue: fmt.Sprintf("%s:%s", row.GroupTitle, row.TagTitle),
			mangaCount: row.MangaCount,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		switch rule.Preset {
		case "genres_za":
			return strings.ToLower(a.label) > strings.ToLower(b.label)
		case "genres_top":
			if a.mangaCount != b.mangaCount {
				return a.mangaCount > b.mangaCount
			}
			return strings.ToLower(a.label) < strings.ToLower(b.label)
		default:
			return strings.ToLower(a.label) < strings.ToLower(b.label)
		}
	})

	if len(entries) > rule.Limit {
		entries = entries[:rule.Limit]
	}

	items := make([]model.DiscoverSectionItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, model.DiscoverSectionItem{
			ID:   liveDiscoverItemID("genres", entry.queryValue),
			Type: "genresCarouselItem",
			Name: entry.label,
			SearchQuery: &model.DiscoverSearchQuery{
				Title:   entry.label,
				Filters: []model.SearchFilter{{ID: "tags", Value: json.RawMessage(strconv.Quote(entry.queryValue))}},
			},
		})
	}
	return items, nil
}

func discoverMangaSubtitle(manga model.Manga) string {
	if manga.Author != "" {
		return manga.Author
	}
	if manga.Artist != "" {
		return manga.Artist
	}
	return fmt.Sprintf("%d chapter%s", manga.ChapterCount, pluralSuffix(manga.ChapterCount))
}

func discoverChapterSubtitle(title string, chapNum float64) string {
	_ = chapNum
	return strings.TrimSpace(title)
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func liveDiscoverItemID(prefix string, key string) string {
	return prefix + ":" + key
}
