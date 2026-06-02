package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"

	"github.com/google/uuid"
)

func (s *Store) SearchManga(ctx context.Context, options model.MangaSearchOptions) ([]model.Manga, error) {
	var search any
	if strings.TrimSpace(options.Query) != "" {
		search = strings.TrimSpace(options.Query)
	}
	sortOption := normalizeMangaSort(options.Sort)
	rows, err := s.queries.SearchMangaSummaries(ctx, gen.SearchMangaSummariesParams{
		Query:          search,
		ContentRatings: jsonArrayOrNil(options.ContentRating),
		Statuses:       jsonArrayOrNil(options.Status),
		MinRating:      floatOrNil(options.MinRating),
		MaxRating:      floatOrNil(options.MaxRating),
		Tags:           jsonArrayOrNil(options.Tags),
		Sort:           sortOption,
	})
	if err != nil {
		return nil, fmt.Errorf("search manga summaries: %w", err)
	}
	items, err := s.buildMangaList(ctx, rows)
	if err != nil {
		return nil, fmt.Errorf("build manga list: %w", err)
	}
	sortMangaByAdjustedChapterCount(items, sortOption)
	return items, nil
}

func (s *Store) GetManga(ctx context.Context, id string) (*model.Manga, error) {
	return s.loadManga(ctx, id)
}

func (s *Store) CreateManga(ctx context.Context, payload model.MangaPayload) (*model.Manga, error) {
	payload = model.NormalizeMangaPayload(payload)
	if payload.PrimaryTitle == "" {
		return nil, fmt.Errorf("%w: primary title is required", ErrValidation)
	}

	id := uuid.NewString()
	now := nowUnix()
	err := s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.CreateManga(ctx, gen.CreateMangaParams{
			ID:            id,
			PrimaryTitle:  payload.PrimaryTitle,
			Synopsis:      payload.Synopsis,
			ThumbnailUrl:  payload.ThumbnailURL,
			BannerUrl:     payload.BannerURL,
			ContentRating: payload.ContentRating,
			Status:        payload.Status,
			Artist:        payload.Artist,
			Author:        payload.Author,
			Rating:        nullFloat(payload.Rating),
			ShareUrl:      payload.ShareURL,
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
		return replaceMangaRelations(ctx, q, id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.loadManga(ctx, id)
}

func (s *Store) UpdateManga(ctx context.Context, id string, payload model.MangaPayload) (*model.Manga, error) {
	payload = model.NormalizeMangaPayload(payload)
	if payload.PrimaryTitle == "" {
		return nil, fmt.Errorf("%w: primary title is required", ErrValidation)
	}
	if _, err := s.queries.GetMangaByID(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	err := s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.UpdateManga(ctx, gen.UpdateMangaParams{
			PrimaryTitle:  payload.PrimaryTitle,
			Synopsis:      payload.Synopsis,
			ThumbnailUrl:  payload.ThumbnailURL,
			BannerUrl:     payload.BannerURL,
			ContentRating: payload.ContentRating,
			Status:        payload.Status,
			Artist:        payload.Artist,
			Author:        payload.Author,
			Rating:        nullFloat(payload.Rating),
			ShareUrl:      payload.ShareURL,
			UpdatedAt:     nowUnix(),
			ID:            id,
		}); err != nil {
			return err
		}
		return replaceMangaRelations(ctx, q, id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.loadManga(ctx, id)
}

func (s *Store) DeleteManga(ctx context.Context, id string) error {
	if _, err := s.queries.GetMangaByID(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.DeleteDiscoverItemsByMangaID(ctx, nullString(id)); err != nil {
			return err
		}
		return q.DeleteManga(ctx, id)
	})
}

func (s *Store) loadManga(ctx context.Context, id string) (*model.Manga, error) {
	record, err := s.queries.GetMangaByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	titles, err := s.queries.ListMangaTitles(ctx, id)
	if err != nil {
		return nil, err
	}
	artwork, err := s.queries.ListMangaArtwork(ctx, id)
	if err != nil {
		return nil, err
	}
	infoEntries, err := s.queries.ListMangaInfoEntries(ctx, id)
	if err != nil {
		return nil, err
	}
	tagRows, err := s.queries.ListMangaTagsJoined(ctx, id)
	if err != nil {
		return nil, err
	}
	chapters, err := s.queries.ListStoredChapters(ctx, id)
	if err != nil {
		return nil, err
	}
	proxyCounts, err := s.proxyChapterCountAdjustments(ctx, []string{id})
	if err != nil {
		return nil, err
	}

	secondaryTitles := make([]string, 0, len(titles))
	for _, title := range titles {
		secondaryTitles = append(secondaryTitles, title.Title)
	}
	artworkURLs := make([]string, 0, len(artwork))
	for _, item := range artwork {
		artworkURLs = append(artworkURLs, item.ImageUrl)
	}
	additionalInfo := make([]model.InfoEntry, 0, len(infoEntries))
	for _, entry := range infoEntries {
		additionalInfo = append(additionalInfo, model.InfoEntry{Key: entry.InfoKey, Value: entry.InfoValue})
	}

	tagGroups := make([]model.TagGroup, 0)
	indexByGroup := map[string]int{}
	for _, row := range tagRows {
		idx, ok := indexByGroup[row.GroupID]
		if !ok {
			tagGroups = append(tagGroups, model.TagGroup{
				ID:    row.GroupID,
				Title: row.GroupTitle,
				Tags:  []model.Tag{},
			})
			idx = len(tagGroups) - 1
			indexByGroup[row.GroupID] = idx
		}
		if row.TagID.Valid {
			tagGroups[idx].Tags = append(tagGroups[idx].Tags, model.Tag{
				ID:    row.TagID.String,
				Title: row.TagTitle.String,
			})
		}
	}

	return &model.Manga{
		ID:              record.ID,
		PrimaryTitle:    record.PrimaryTitle,
		SecondaryTitles: secondaryTitles,
		Synopsis:        record.Synopsis,
		ThumbnailURL:    record.ThumbnailUrl,
		BannerURL:       record.BannerUrl,
		ContentRating:   record.ContentRating,
		Status:          record.Status,
		Artist:          record.Artist,
		Author:          record.Author,
		Rating:          floatPtrFromNull(record.Rating),
		ShareURL:        record.ShareUrl,
		ArtworkURLs:     artworkURLs,
		TagGroups:       tagGroups,
		AdditionalInfo:  additionalInfo,
		ChapterCount:    len(chapters) + proxyCounts[id],
		CreatedAt:       isoString(record.CreatedAt),
		UpdatedAt:       isoString(record.UpdatedAt),
	}, nil
}

func replaceMangaRelations(ctx context.Context, q *gen.Queries, mangaID string, payload model.MangaPayload) error {
	if err := q.DeleteMangaTitles(ctx, mangaID); err != nil {
		return err
	}
	if err := q.DeleteMangaArtwork(ctx, mangaID); err != nil {
		return err
	}
	if err := q.DeleteMangaTagGroups(ctx, mangaID); err != nil {
		return err
	}
	if err := q.DeleteMangaInfoEntries(ctx, mangaID); err != nil {
		return err
	}

	for index, title := range payload.SecondaryTitles {
		if err := q.InsertMangaTitle(ctx, gen.InsertMangaTitleParams{
			ID:        uuid.NewString(),
			MangaID:   mangaID,
			Title:     title,
			TitleType: "secondary",
			SortOrder: int64(index),
		}); err != nil {
			return err
		}
	}
	for index, url := range payload.ArtworkURLs {
		if err := q.InsertMangaArtwork(ctx, gen.InsertMangaArtworkParams{
			ID:        uuid.NewString(),
			MangaID:   mangaID,
			ImageUrl:  url,
			SortOrder: int64(index),
		}); err != nil {
			return err
		}
	}
	for groupIndex, group := range payload.TagGroups {
		groupID := uuid.NewString()
		if err := q.InsertMangaTagGroup(ctx, gen.InsertMangaTagGroupParams{
			ID:        groupID,
			MangaID:   mangaID,
			Title:     group.Title,
			SortOrder: int64(groupIndex),
		}); err != nil {
			return err
		}
		for tagIndex, tag := range group.Tags {
			if err := q.InsertMangaTag(ctx, gen.InsertMangaTagParams{
				ID:         uuid.NewString(),
				TagGroupID: groupID,
				Title:      tag.Title,
				SortOrder:  int64(tagIndex),
			}); err != nil {
				return err
			}
		}
	}
	for index, entry := range payload.AdditionalInfo {
		if err := q.InsertMangaInfoEntry(ctx, gen.InsertMangaInfoEntryParams{
			ID:        uuid.NewString(),
			MangaID:   mangaID,
			InfoKey:   entry.Key,
			InfoValue: entry.Value,
			SortOrder: int64(index),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) buildMangaList(ctx context.Context, rows []gen.SearchMangaSummariesRow) ([]model.Manga, error) {
	if len(rows) == 0 {
		return []model.Manga{}, nil
	}

	ids := make([]string, 0, len(rows))
	indexByID := make(map[string]int, len(rows))
	result := make([]model.Manga, 0, len(rows))
	for _, row := range rows {
		indexByID[row.ID] = len(result)
		ids = append(ids, row.ID)
		result = append(result, model.Manga{
			ID:              row.ID,
			PrimaryTitle:    row.PrimaryTitle,
			SecondaryTitles: []string{},
			Synopsis:        row.Synopsis,
			ThumbnailURL:    row.ThumbnailUrl,
			BannerURL:       row.BannerUrl,
			ContentRating:   row.ContentRating,
			Status:          row.Status,
			Artist:          row.Artist,
			Author:          row.Author,
			Rating:          floatPtrFromNull(row.Rating),
			ShareURL:        row.ShareUrl,
			ArtworkURLs:     []string{},
			TagGroups:       []model.TagGroup{},
			AdditionalInfo:  []model.InfoEntry{},
			ChapterCount:    int(row.ChapterCount),
			CreatedAt:       isoString(row.CreatedAt),
			UpdatedAt:       isoString(row.UpdatedAt),
		})
	}
	proxyCounts, err := s.proxyChapterCountAdjustments(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list proxy chapter counts: %w", err)
	}
	for id, count := range proxyCounts {
		if index, ok := indexByID[id]; ok {
			result[index].ChapterCount += count
		}
	}

	titles, err := s.queries.ListMangaTitlesByMangaIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list manga titles by ids: %w", err)
	}
	for _, title := range titles {
		index := indexByID[title.MangaID]
		result[index].SecondaryTitles = append(result[index].SecondaryTitles, title.Title)
	}

	artwork, err := s.queries.ListMangaArtworkByMangaIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list manga artwork by ids: %w", err)
	}
	for _, item := range artwork {
		index := indexByID[item.MangaID]
		result[index].ArtworkURLs = append(result[index].ArtworkURLs, item.ImageUrl)
	}

	infoEntries, err := s.queries.ListMangaInfoEntriesByMangaIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list manga info entries by ids: %w", err)
	}
	for _, entry := range infoEntries {
		index := indexByID[entry.MangaID]
		result[index].AdditionalInfo = append(result[index].AdditionalInfo, model.InfoEntry{
			Key:   entry.InfoKey,
			Value: entry.InfoValue,
		})
	}

	tagRows, err := s.queries.ListMangaTagsJoinedByMangaIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list manga tags by ids: %w", err)
	}
	groupIndexByManga := make(map[string]map[string]int, len(result))
	for _, row := range tagRows {
		index := indexByID[row.MangaID]
		groupIndex, ok := groupIndexByManga[row.MangaID]
		if !ok {
			groupIndex = make(map[string]int)
			groupIndexByManga[row.MangaID] = groupIndex
		}

		tagGroupIndex, ok := groupIndex[row.GroupID]
		if !ok {
			result[index].TagGroups = append(result[index].TagGroups, model.TagGroup{
				ID:    row.GroupID,
				Title: row.GroupTitle,
				Tags:  []model.Tag{},
			})
			tagGroupIndex = len(result[index].TagGroups) - 1
			groupIndex[row.GroupID] = tagGroupIndex
		}
		if row.TagID.Valid {
			result[index].TagGroups[tagGroupIndex].Tags = append(result[index].TagGroups[tagGroupIndex].Tags, model.Tag{
				ID:    row.TagID.String,
				Title: row.TagTitle.String,
			})
		}
	}

	return result, nil
}

func (s *Store) proxyChapterCountAdjustments(ctx context.Context, mangaIDs []string) (map[string]int, error) {
	if len(mangaIDs) == 0 {
		return map[string]int{}, nil
	}
	rows, err := s.queries.ListProxyChapterCountAdjustments(ctx, mangaIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		if row.ProxyChapterCount.Valid {
			result[row.MangaID] = int(row.ProxyChapterCount.Float64)
		}
	}
	return result, nil
}

func sortMangaByAdjustedChapterCount(items []model.Manga, sortOption string) {
	switch sortOption {
	case "chapters_asc":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].ChapterCount != items[j].ChapterCount {
				return items[i].ChapterCount < items[j].ChapterCount
			}
			leftTitle := strings.ToLower(items[i].PrimaryTitle)
			rightTitle := strings.ToLower(items[j].PrimaryTitle)
			if leftTitle != rightTitle {
				return leftTitle < rightTitle
			}
			return items[i].ID < items[j].ID
		})
	case "chapters_desc":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].ChapterCount != items[j].ChapterCount {
				return items[i].ChapterCount > items[j].ChapterCount
			}
			leftTitle := strings.ToLower(items[i].PrimaryTitle)
			rightTitle := strings.ToLower(items[j].PrimaryTitle)
			if leftTitle != rightTitle {
				return leftTitle < rightTitle
			}
			return items[i].ID < items[j].ID
		})
	}
}

func jsonArrayOrNil(values []string) any {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	encoded, _ := json.Marshal(cleaned)
	return string(encoded)
}

func floatOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func normalizeMangaSort(value string) string {
	switch value {
	case "updated_asc", "title_asc", "title_desc", "rating_desc", "rating_asc", "chapters_desc", "chapters_asc":
		return value
	default:
		return "updated_desc"
	}
}
