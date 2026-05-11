package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func (s *Store) ExportBackup(ctx context.Context) (*model.Backup, error) {
	manga, err := s.SearchManga(ctx, model.MangaSearchOptions{})
	if err != nil {
		return nil, fmt.Errorf("list manga for backup: %w", err)
	}

	chapters := make([]model.BackupChapter, 0)
	chapterSources := make([]model.BackupChapterSource, 0)
	backupManga := make([]model.BackupManga, 0, len(manga))

	for _, item := range manga {
		backupManga = append(backupManga, model.BackupManga{
			ID:              item.ID,
			PrimaryTitle:    item.PrimaryTitle,
			SecondaryTitles: append([]string{}, item.SecondaryTitles...),
			Synopsis:        item.Synopsis,
			ThumbnailURL:    item.ThumbnailURL,
			BannerURL:       item.BannerURL,
			ContentRating:   item.ContentRating,
			Status:          item.Status,
			Artist:          item.Artist,
			Author:          item.Author,
			Rating:          item.Rating,
			ShareURL:        item.ShareURL,
			ArtworkURLs:     append([]string{}, item.ArtworkURLs...),
			TagGroups:       cloneTagGroups(item.TagGroups),
			AdditionalInfo:  cloneInfoEntries(item.AdditionalInfo),
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})

		storedChapters, err := s.listStoredChapters(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list stored chapters for manga %s: %w", item.ID, err)
		}
		for _, chapter := range storedChapters {
			detail, err := s.getStoredChapter(ctx, chapter.ID)
			if err != nil {
				return nil, fmt.Errorf("load chapter %s for backup: %w", chapter.ID, err)
			}
			chapters = append(chapters, model.BackupChapter{
				ID:             detail.ID,
				MangaID:        detail.MangaID,
				LangCode:       detail.LangCode,
				ChapNum:        detail.ChapNum,
				Title:          detail.Title,
				Version:        detail.Version,
				Volume:         detail.Volume,
				PublishDate:    detail.PublishDate,
				CreationDate:   detail.CreationDate,
				SortingIndex:   detail.SortingIndex,
				AdditionalInfo: cloneInfoEntries(detail.AdditionalInfo),
				Pages:          append([]string{}, detail.Pages...),
				LastUpdated:    detail.LastUpdated,
				Origin:         detail.Origin,
			})
		}

		sourceRows, err := s.queries.ListChapterSources(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list chapter sources for manga %s: %w", item.ID, err)
		}
		for _, row := range sourceRows {
			chapterSources = append(chapterSources, model.BackupChapterSource{
				ID:                  row.ID,
				MangaID:             row.MangaID,
				Provider:            row.Provider,
				Mode:                row.Mode,
				Config:              json.RawMessage(row.ConfigJson),
				SyncIntervalMinutes: int(row.SyncIntervalMinutes),
				CreatedAt:           isoString(row.CreatedAt),
				UpdatedAt:           isoString(row.UpdatedAt),
			})
		}
	}

	collections, err := s.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections for backup: %w", err)
	}
	backupCollections := make([]model.BackupCollection, 0, len(collections))
	for _, item := range collections {
		collectionManga, err := s.queries.ListCollectionMangaSummaries(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list collection manga for collection %s: %w", item.ID, err)
		}
		mangaIDs := make([]string, 0, len(collectionManga))
		for _, entry := range collectionManga {
			mangaIDs = append(mangaIDs, entry.ID)
		}
		backupCollections = append(backupCollections, model.BackupCollection{
			ID:        item.ID,
			Title:     item.Title,
			SortOrder: item.SortOrder,
			MangaIDs:  mangaIDs,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	discoverSections, err := s.listStoredDiscoverSectionConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list discover sections for backup: %w", err)
	}
	backupDiscoverSections := make([]model.BackupDiscoverSection, 0, len(discoverSections))
	for _, section := range discoverSections {
		backupDiscoverSections = append(backupDiscoverSections, model.BackupDiscoverSection{
			ID:        section.ID,
			Title:     section.Title,
			Subtitle:  section.Subtitle,
			Type:      section.Type,
			Mode:      section.Mode,
			LiveRule:  cloneDiscoverLiveRule(section.LiveRule),
			SortOrder: section.SortOrder,
			Items:     cloneDiscoverItems(section.Items),
		})
	}

	return &model.Backup{
		SchemaVersion:    model.BackupSchemaVersion,
		Manga:            backupManga,
		Chapters:         chapters,
		Collections:      backupCollections,
		DiscoverSections: backupDiscoverSections,
		ChapterSources:   chapterSources,
	}, nil
}

func cloneInfoEntries(entries []model.InfoEntry) []model.InfoEntry {
	result := make([]model.InfoEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, model.InfoEntry{
			Key:   entry.Key,
			Value: entry.Value,
		})
	}
	return result
}

func cloneTagGroups(groups []model.TagGroup) []model.TagGroup {
	result := make([]model.TagGroup, 0, len(groups))
	for _, group := range groups {
		tags := make([]model.Tag, 0, len(group.Tags))
		for _, tag := range group.Tags {
			tags = append(tags, model.Tag{
				ID:    tag.ID,
				Title: tag.Title,
			})
		}
		result = append(result, model.TagGroup{
			ID:    group.ID,
			Title: group.Title,
			Tags:  tags,
		})
	}
	return result
}

func cloneDiscoverLiveRule(rule *model.DiscoverLiveRule) *model.DiscoverLiveRule {
	if rule == nil {
		return nil
	}
	cloned := *rule
	return &cloned
}

func cloneDiscoverItems(items []model.DiscoverSectionItem) []model.DiscoverSectionItem {
	result := make([]model.DiscoverSectionItem, 0, len(items))
	for _, item := range items {
		cloned := model.DiscoverSectionItem{
			ID:          item.ID,
			Type:        item.Type,
			MangaID:     item.MangaID,
			ChapterID:   item.ChapterID,
			ImageURL:    item.ImageURL,
			Title:       item.Title,
			Subtitle:    item.Subtitle,
			Supertitle:  item.Supertitle,
			Name:        item.Name,
			PublishDate: item.PublishDate,
		}
		if item.ChapNum != nil {
			value := *item.ChapNum
			cloned.ChapNum = &value
		}
		if item.ContentRating != nil {
			value := *item.ContentRating
			cloned.ContentRating = &value
		}
		if item.Metadata != nil {
			cloned.Metadata = append(json.RawMessage{}, item.Metadata...)
		}
		if item.SearchQuery != nil {
			filters := make([]model.SearchFilter, 0, len(item.SearchQuery.Filters))
			for _, filter := range item.SearchQuery.Filters {
				filters = append(filters, model.SearchFilter{
					ID:    filter.ID,
					Value: append(json.RawMessage{}, filter.Value...),
				})
			}
			cloned.SearchQuery = &model.DiscoverSearchQuery{
				Title:   item.SearchQuery.Title,
				Filters: filters,
			}
		}
		result = append(result, cloned)
	}
	return result
}
