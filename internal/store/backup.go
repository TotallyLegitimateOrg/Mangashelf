package store

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/buildinfo"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func (s *Store) ExportBackup(ctx context.Context) (*model.Backup, error) {
	manga, err := s.SearchManga(ctx, model.MangaSearchOptions{Sort: "title_asc"})
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

const (
	backupFormat       = "mangashelf.backup"
	backupManifestPath = "manifest.json"
)

var backupZipTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func (s *Store) ExportBackupArchive(ctx context.Context, w io.Writer) error {
	backup, err := s.ExportBackup(ctx)
	if err != nil {
		return err
	}
	normalizeBackupJSON(backup)
	if err := validateBackup(backup); err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	info := buildinfo.Current()

	manifest := model.BackupManifest{
		Format:        backupFormat,
		SchemaVersion: model.BackupSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		App: model.BackupManifestApp{
			Version: info.Version,
			Commit:  info.Commit,
			BuiltAt: info.BuiltAt,
		},
		Counts: model.BackupCounts{
			Manga:            len(backup.Manga),
			Chapters:         len(backup.Chapters),
			ChapterSources:   len(backup.ChapterSources),
			Collections:      len(backup.Collections),
			DiscoverSections: len(backup.DiscoverSections),
		},
	}
	if err := writeBackupJSONFile(zw, backupManifestPath, manifest); err != nil {
		return err
	}

	sort.Slice(backup.Manga, func(i, j int) bool { return backup.Manga[i].ID < backup.Manga[j].ID })
	for _, item := range backup.Manga {
		if err := writeBackupJSONFile(zw, "manga/"+item.ID+".json", item); err != nil {
			return err
		}
	}
	sort.Slice(backup.Chapters, func(i, j int) bool { return backup.Chapters[i].ID < backup.Chapters[j].ID })
	for _, item := range backup.Chapters {
		if err := writeBackupJSONFile(zw, "chapters/"+item.ID+".json", item); err != nil {
			return err
		}
	}
	sort.Slice(backup.ChapterSources, func(i, j int) bool { return backup.ChapterSources[i].ID < backup.ChapterSources[j].ID })
	for _, item := range backup.ChapterSources {
		if err := writeBackupJSONFile(zw, "chapter-sources/"+item.ID+".json", item); err != nil {
			return err
		}
	}
	sort.Slice(backup.Collections, func(i, j int) bool { return backup.Collections[i].ID < backup.Collections[j].ID })
	for _, item := range backup.Collections {
		if err := writeBackupJSONFile(zw, "collections/"+item.ID+".json", item); err != nil {
			return err
		}
	}
	sort.Slice(backup.DiscoverSections, func(i, j int) bool { return backup.DiscoverSections[i].ID < backup.DiscoverSections[j].ID })
	for _, item := range backup.DiscoverSections {
		if err := writeBackupJSONFile(zw, "discover-sections/"+item.ID+".json", item); err != nil {
			return err
		}
	}
	return zw.Close()
}

func (s *Store) RestoreBackupArchive(ctx context.Context, r io.Reader) (*model.BackupRestoreResult, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read backup archive: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("%w: backup must be a valid zip archive", ErrValidation)
	}

	backup := &model.Backup{SchemaVersion: model.BackupSchemaVersion}
	var manifest *model.BackupManifest
	seen := map[string]struct{}{}
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		clean := path.Clean(file.Name)
		if clean != file.Name || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return nil, fmt.Errorf("%w: invalid backup path %q", ErrValidation, file.Name)
		}
		if _, ok := seen[clean]; ok {
			return nil, fmt.Errorf("%w: duplicate backup entry %s", ErrValidation, clean)
		}
		seen[clean] = struct{}{}

		switch {
		case clean == backupManifestPath:
			var parsed model.BackupManifest
			if err := readBackupZipJSON(file, &parsed); err != nil {
				return nil, err
			}
			manifest = &parsed
		case strings.HasPrefix(clean, "manga/"):
			var item model.BackupManga
			if err := readBackupEntity(file, clean, "manga", &item.ID, &item); err != nil {
				return nil, err
			}
			backup.Manga = append(backup.Manga, item)
		case strings.HasPrefix(clean, "chapters/"):
			var item model.BackupChapter
			if err := readBackupEntity(file, clean, "chapters", &item.ID, &item); err != nil {
				return nil, err
			}
			backup.Chapters = append(backup.Chapters, item)
		case strings.HasPrefix(clean, "chapter-sources/"):
			var item model.BackupChapterSource
			if err := readBackupEntity(file, clean, "chapter-sources", &item.ID, &item); err != nil {
				return nil, err
			}
			backup.ChapterSources = append(backup.ChapterSources, item)
		case strings.HasPrefix(clean, "collections/"):
			var item model.BackupCollection
			if err := readBackupEntity(file, clean, "collections", &item.ID, &item); err != nil {
				return nil, err
			}
			backup.Collections = append(backup.Collections, item)
		case strings.HasPrefix(clean, "discover-sections/"):
			var item model.BackupDiscoverSection
			if err := readBackupEntity(file, clean, "discover-sections", &item.ID, &item); err != nil {
				return nil, err
			}
			backup.DiscoverSections = append(backup.DiscoverSections, item)
		default:
			return nil, fmt.Errorf("%w: unknown backup entry %s", ErrValidation, clean)
		}
	}
	if manifest == nil {
		return nil, fmt.Errorf("%w: backup manifest is required", ErrValidation)
	}
	if manifest.Format != backupFormat {
		return nil, fmt.Errorf("%w: unsupported backup format %q", ErrValidation, manifest.Format)
	}
	if manifest.SchemaVersion != model.BackupSchemaVersion {
		return nil, fmt.Errorf("%w: unsupported backup schema version %d", ErrValidation, manifest.SchemaVersion)
	}
	if err := validateBackupCounts(manifest.Counts, backup); err != nil {
		return nil, err
	}
	normalizeBackupJSON(backup)
	return s.RestoreBackup(ctx, backup)
}

func writeBackupJSONFile(zw *zip.Writer, name string, value any) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetModTime(backupZipTime)
	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create backup entry %s: %w", name, err)
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write backup entry %s: %w", name, err)
	}
	return nil
}

func readBackupZipJSON(file *zip.File, target any) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open backup entry %s: %w", file.Name, err)
	}
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid JSON in %s", ErrValidation, file.Name)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: trailing JSON in %s", ErrValidation, file.Name)
	}
	return nil
}

func readBackupEntity(file *zip.File, clean string, dir string, id *string, target any) error {
	if path.Dir(clean) != dir || path.Ext(clean) != ".json" {
		return fmt.Errorf("%w: invalid backup entry %s", ErrValidation, clean)
	}
	wantID := strings.TrimSuffix(path.Base(clean), ".json")
	if wantID == "" || strings.Contains(wantID, "/") {
		return fmt.Errorf("%w: invalid backup entry %s", ErrValidation, clean)
	}
	if err := readBackupZipJSON(file, target); err != nil {
		return err
	}
	if *id != wantID {
		return fmt.Errorf("%w: backup entry %s contains id %q", ErrValidation, clean, *id)
	}
	return nil
}

func validateBackupCounts(counts model.BackupCounts, backup *model.Backup) error {
	if counts.Manga != len(backup.Manga) {
		return fmt.Errorf("%w: manifest manga count mismatch", ErrValidation)
	}
	if counts.Chapters != len(backup.Chapters) {
		return fmt.Errorf("%w: manifest chapter count mismatch", ErrValidation)
	}
	if counts.ChapterSources != len(backup.ChapterSources) {
		return fmt.Errorf("%w: manifest chapter source count mismatch", ErrValidation)
	}
	if counts.Collections != len(backup.Collections) {
		return fmt.Errorf("%w: manifest collection count mismatch", ErrValidation)
	}
	if counts.DiscoverSections != len(backup.DiscoverSections) {
		return fmt.Errorf("%w: manifest discover section count mismatch", ErrValidation)
	}
	return nil
}

func normalizeBackupJSON(backup *model.Backup) {
	for index := range backup.ChapterSources {
		backup.ChapterSources[index].Config = normalizeRawJSON(backup.ChapterSources[index].Config)
	}
	for sectionIndex := range backup.DiscoverSections {
		for itemIndex := range backup.DiscoverSections[sectionIndex].Items {
			item := &backup.DiscoverSections[sectionIndex].Items[itemIndex]
			item.Metadata = normalizeRawJSON(item.Metadata)
		}
	}
}

func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return normalized
}

func (s *Store) RestoreBackup(ctx context.Context, backup *model.Backup) (*model.BackupRestoreResult, error) {
	if backup == nil {
		return nil, fmt.Errorf("%w: backup is required", ErrValidation)
	}
	if backup.SchemaVersion != model.BackupSchemaVersion {
		return nil, fmt.Errorf("%w: unsupported backup schema version %d", ErrValidation, backup.SchemaVersion)
	}
	if err := validateBackup(backup); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := clearRestorableData(ctx, tx); err != nil {
		return nil, err
	}
	if err := insertBackupManga(ctx, tx, backup.Manga); err != nil {
		return nil, err
	}
	if err := insertBackupChapterSources(ctx, tx, backup.ChapterSources); err != nil {
		return nil, err
	}
	if err := insertBackupChapters(ctx, tx, backup.Chapters); err != nil {
		return nil, err
	}
	if err := insertBackupCollections(ctx, tx, backup.Collections); err != nil {
		return nil, err
	}
	if err := insertBackupDiscoverSections(ctx, tx, backup.DiscoverSections); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.BackupRestoreResult{
		MangaCount:           len(backup.Manga),
		ChapterCount:         len(backup.Chapters),
		CollectionCount:      len(backup.Collections),
		DiscoverSectionCount: len(backup.DiscoverSections),
		ChapterSourceCount:   len(backup.ChapterSources),
	}, nil
}

func validateBackup(backup *model.Backup) error {
	mangaIDs := make(map[string]struct{}, len(backup.Manga))
	chapterIDs := make(map[string]struct{}, len(backup.Chapters))
	sourceIDs := make(map[string]struct{}, len(backup.ChapterSources))

	for _, item := range backup.Manga {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("%w: manga id is required", ErrValidation)
		}
		if !isUUIDv7(item.ID) {
			return fmt.Errorf("%w: manga %s id must be UUIDv7", ErrValidation, item.ID)
		}
		if strings.TrimSpace(item.PrimaryTitle) == "" {
			return fmt.Errorf("%w: manga %s primary title is required", ErrValidation, item.ID)
		}
		if _, exists := mangaIDs[item.ID]; exists {
			return fmt.Errorf("%w: duplicate manga id %s", ErrValidation, item.ID)
		}
		mangaIDs[item.ID] = struct{}{}
		if _, err := parseBackupTime(item.CreatedAt, "manga "+item.ID+" createdAt"); err != nil {
			return err
		}
		if _, err := parseBackupTime(item.UpdatedAt, "manga "+item.ID+" updatedAt"); err != nil {
			return err
		}
	}

	for _, source := range backup.ChapterSources {
		if strings.TrimSpace(source.ID) == "" {
			return fmt.Errorf("%w: chapter source id is required", ErrValidation)
		}
		if !isUUIDv7(source.ID) {
			return fmt.Errorf("%w: chapter source %s id must be UUIDv7", ErrValidation, source.ID)
		}
		if _, exists := sourceIDs[source.ID]; exists {
			return fmt.Errorf("%w: duplicate chapter source id %s", ErrValidation, source.ID)
		}
		sourceIDs[source.ID] = struct{}{}
		if _, ok := mangaIDs[source.MangaID]; !ok {
			return fmt.Errorf("%w: chapter source %s references missing manga %s", ErrValidation, source.ID, source.MangaID)
		}
		if strings.TrimSpace(source.Provider) == "" {
			return fmt.Errorf("%w: chapter source %s provider is required", ErrValidation, source.ID)
		}
		if strings.TrimSpace(source.Mode) == "" {
			return fmt.Errorf("%w: chapter source %s mode is required", ErrValidation, source.ID)
		}
		if !json.Valid(source.Config) {
			return fmt.Errorf("%w: chapter source %s config must be valid JSON", ErrValidation, source.ID)
		}
		if _, err := parseBackupTime(source.CreatedAt, "chapter source "+source.ID+" createdAt"); err != nil {
			return err
		}
		if _, err := parseBackupTime(source.UpdatedAt, "chapter source "+source.ID+" updatedAt"); err != nil {
			return err
		}
	}

	for _, chapter := range backup.Chapters {
		if strings.TrimSpace(chapter.ID) == "" {
			return fmt.Errorf("%w: chapter id is required", ErrValidation)
		}
		if !isUUIDv7(chapter.ID) {
			return fmt.Errorf("%w: chapter %s id must be UUIDv7", ErrValidation, chapter.ID)
		}
		if _, exists := chapterIDs[chapter.ID]; exists {
			return fmt.Errorf("%w: duplicate chapter id %s", ErrValidation, chapter.ID)
		}
		chapterIDs[chapter.ID] = struct{}{}
		if _, ok := mangaIDs[chapter.MangaID]; !ok {
			return fmt.Errorf("%w: chapter %s references missing manga %s", ErrValidation, chapter.ID, chapter.MangaID)
		}
		if chapter.ChapNum == 0 {
			return fmt.Errorf("%w: chapter %s number is required", ErrValidation, chapter.ID)
		}
		if _, err := parseBackupOptionalTime(chapter.PublishDate, "chapter "+chapter.ID+" publishDate"); err != nil {
			return err
		}
		if _, err := parseBackupOptionalTime(chapter.CreationDate, "chapter "+chapter.ID+" creationDate"); err != nil {
			return err
		}
		if _, err := parseBackupTime(chapter.LastUpdated, "chapter "+chapter.ID+" lastUpdated"); err != nil {
			return err
		}
		if chapter.Origin.SourceID != nil && strings.TrimSpace(*chapter.Origin.SourceID) != "" {
			if _, ok := sourceIDs[*chapter.Origin.SourceID]; !ok {
				return fmt.Errorf("%w: chapter %s references missing source %s", ErrValidation, chapter.ID, *chapter.Origin.SourceID)
			}
		}
	}

	for _, collection := range backup.Collections {
		if strings.TrimSpace(collection.ID) == "" {
			return fmt.Errorf("%w: collection id is required", ErrValidation)
		}
		if !isUUIDv7(collection.ID) {
			return fmt.Errorf("%w: collection %s id must be UUIDv7", ErrValidation, collection.ID)
		}
		if strings.TrimSpace(collection.Title) == "" {
			return fmt.Errorf("%w: collection %s title is required", ErrValidation, collection.ID)
		}
		if _, err := parseBackupTime(collection.CreatedAt, "collection "+collection.ID+" createdAt"); err != nil {
			return err
		}
		if _, err := parseBackupTime(collection.UpdatedAt, "collection "+collection.ID+" updatedAt"); err != nil {
			return err
		}
		for _, mangaID := range collection.MangaIDs {
			if _, ok := mangaIDs[mangaID]; !ok {
				return fmt.Errorf("%w: collection %s references missing manga %s", ErrValidation, collection.ID, mangaID)
			}
		}
	}

	for _, section := range backup.DiscoverSections {
		if strings.TrimSpace(section.ID) == "" {
			return fmt.Errorf("%w: discover section id is required", ErrValidation)
		}
		if !isUUIDv7(section.ID) {
			return fmt.Errorf("%w: discover section %s id must be UUIDv7", ErrValidation, section.ID)
		}
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf("%w: discover section %s title is required", ErrValidation, section.ID)
		}
		if section.LiveRule != nil && len(section.Items) > 0 {
			return fmt.Errorf("%w: live discover section %s must not include derived items", ErrValidation, section.ID)
		}
		for _, item := range section.Items {
			if strings.TrimSpace(item.ID) == "" {
				return fmt.Errorf("%w: discover item id is required", ErrValidation)
			}
			if !isUUIDv7(item.ID) {
				return fmt.Errorf("%w: discover item %s id must be UUIDv7", ErrValidation, item.ID)
			}
			if item.MangaID != "" {
				if _, ok := mangaIDs[item.MangaID]; !ok {
					return fmt.Errorf("%w: discover item %s references missing manga %s", ErrValidation, item.ID, item.MangaID)
				}
			}
			if item.ChapterID != "" {
				if _, ok := chapterIDs[item.ChapterID]; !ok {
					return fmt.Errorf("%w: discover item %s references missing chapter %s", ErrValidation, item.ID, item.ChapterID)
				}
			}
			if item.PublishDate != nil {
				if _, err := parseBackupOptionalTime(item.PublishDate, "discover item "+item.ID+" publishDate"); err != nil {
					return err
				}
			}
			if item.Metadata != nil && !json.Valid(item.Metadata) {
				return fmt.Errorf("%w: discover item %s metadata must be valid JSON", ErrValidation, item.ID)
			}
		}
	}
	return nil
}

func clearRestorableData(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		"DELETE FROM discover_items",
		"DELETE FROM discover_sections",
		"DELETE FROM collection_manga",
		"DELETE FROM collections",
		"DELETE FROM chapter_source_sync_logs",
		"DELETE FROM chapter_sources",
		"DELETE FROM chapter_info_entries",
		"DELETE FROM chapter_pages",
		"DELETE FROM chapters",
		"DELETE FROM manga_info_entries",
		"DELETE FROM manga_tags",
		"DELETE FROM manga_tag_groups",
		"DELETE FROM manga_artwork",
		"DELETE FROM manga_titles",
		"DELETE FROM manga",
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("clear restorable data: %w", err)
		}
	}
	return nil
}

func insertBackupManga(ctx context.Context, tx *sql.Tx, manga []model.BackupManga) error {
	for _, item := range manga {
		createdAt, _ := parseBackupTime(item.CreatedAt, "")
		updatedAt, _ := parseBackupTime(item.UpdatedAt, "")
		if _, err := tx.ExecContext(ctx, `
INSERT INTO manga (
  id, primary_title, synopsis, thumbnail_url, banner_url, content_rating,
  status, artist, author, rating, share_url, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.PrimaryTitle, item.Synopsis, item.ThumbnailURL, item.BannerURL,
			item.ContentRating, item.Status, item.Artist, item.Author, nullFloat(item.Rating),
			item.ShareURL, createdAt, updatedAt,
		); err != nil {
			return fmt.Errorf("restore manga %s: %w", item.ID, err)
		}
		for index, title := range item.SecondaryTitles {
			id, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO manga_titles (id, manga_id, title, title_type, sort_order) VALUES (?, ?, ?, ?, ?)`,
				id, item.ID, title, "secondary", index); err != nil {
				return fmt.Errorf("restore manga %s title: %w", item.ID, err)
			}
		}
		for index, url := range item.ArtworkURLs {
			id, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO manga_artwork (id, manga_id, image_url, sort_order) VALUES (?, ?, ?, ?)`,
				id, item.ID, url, index); err != nil {
				return fmt.Errorf("restore manga %s artwork: %w", item.ID, err)
			}
		}
		for groupIndex, group := range item.TagGroups {
			groupID := group.ID
			if groupID == "" {
				var err error
				groupID, err = newID()
				if err != nil {
					return err
				}
			} else if !isUUIDv7(groupID) {
				return fmt.Errorf("%w: manga %s tag group %s id must be UUIDv7", ErrValidation, item.ID, groupID)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO manga_tag_groups (id, manga_id, title, sort_order) VALUES (?, ?, ?, ?)`,
				groupID, item.ID, group.Title, groupIndex); err != nil {
				return fmt.Errorf("restore manga %s tag group: %w", item.ID, err)
			}
			for tagIndex, tag := range group.Tags {
				tagID := tag.ID
				if tagID == "" {
					var err error
					tagID, err = newID()
					if err != nil {
						return err
					}
				} else if !isUUIDv7(tagID) {
					return fmt.Errorf("%w: manga %s tag %s id must be UUIDv7", ErrValidation, item.ID, tagID)
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO manga_tags (id, tag_group_id, title, sort_order) VALUES (?, ?, ?, ?)`,
					tagID, groupID, tag.Title, tagIndex); err != nil {
					return fmt.Errorf("restore manga %s tag: %w", item.ID, err)
				}
			}
		}
		for index, entry := range item.AdditionalInfo {
			id, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO manga_info_entries (id, manga_id, info_key, info_value, sort_order) VALUES (?, ?, ?, ?, ?)`,
				id, item.ID, entry.Key, entry.Value, index); err != nil {
				return fmt.Errorf("restore manga %s info: %w", item.ID, err)
			}
		}
	}
	return nil
}

func insertBackupChapterSources(ctx context.Context, tx *sql.Tx, sources []model.BackupChapterSource) error {
	for _, source := range sources {
		createdAt, _ := parseBackupTime(source.CreatedAt, "")
		updatedAt, _ := parseBackupTime(source.UpdatedAt, "")
		if _, err := tx.ExecContext(ctx, `
INSERT INTO chapter_sources (
  id, manga_id, provider, mode, config_json, status, last_error,
  last_seen_chapter_count, sync_interval_minutes, last_synced_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			source.ID, source.MangaID, source.Provider, source.Mode, string(source.Config), "ready", "",
			sql.NullInt64{}, source.SyncIntervalMinutes, sql.NullInt64{}, createdAt, updatedAt,
		); err != nil {
			return fmt.Errorf("restore chapter source %s: %w", source.ID, err)
		}
	}
	return nil
}

func insertBackupChapters(ctx context.Context, tx *sql.Tx, chapters []model.BackupChapter) error {
	for _, chapter := range chapters {
		publishDate, _ := parseBackupOptionalTime(chapter.PublishDate, "")
		creationDate, _ := parseBackupOptionalTime(chapter.CreationDate, "")
		lastUpdated, _ := parseBackupTime(chapter.LastUpdated, "")
		if _, err := tx.ExecContext(ctx, `
INSERT INTO chapters (
  id, manga_id, lang_code, chap_num, title, version, volume, publish_date,
  creation_date, sorting_index, origin_provider, origin_mode, origin_source_id,
  origin_source_status, origin_chapter_key, last_updated
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chapter.ID, chapter.MangaID, chapter.LangCode, chapter.ChapNum, chapter.Title, chapter.Version,
			nullFloat(chapter.Volume), nullableUnix(publishDate), nullableUnix(creationDate),
			nullFloat(chapter.SortingIndex), backupNullableStringPtr(chapter.Origin.Provider),
			nullableOriginMode(chapter.Origin), backupNullableStringPtr(chapter.Origin.SourceID),
			backupNullableStringPtr(chapter.Origin.SourceStatus), backupNullableStringPtr(chapter.Origin.ChapterKey),
			lastUpdated,
		); err != nil {
			return fmt.Errorf("restore chapter %s: %w", chapter.ID, err)
		}
		for index, entry := range chapter.AdditionalInfo {
			id, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO chapter_info_entries (id, chapter_id, info_key, info_value, sort_order) VALUES (?, ?, ?, ?, ?)`,
				id, chapter.ID, entry.Key, entry.Value, index); err != nil {
				return fmt.Errorf("restore chapter %s info: %w", chapter.ID, err)
			}
		}
		for index, page := range chapter.Pages {
			id, err := newID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO chapter_pages (id, chapter_id, page_num, image_url) VALUES (?, ?, ?, ?)`,
				id, chapter.ID, index+1, page); err != nil {
				return fmt.Errorf("restore chapter %s page: %w", chapter.ID, err)
			}
		}
	}
	return nil
}

func insertBackupCollections(ctx context.Context, tx *sql.Tx, collections []model.BackupCollection) error {
	for _, collection := range collections {
		createdAt, _ := parseBackupTime(collection.CreatedAt, "")
		updatedAt, _ := parseBackupTime(collection.UpdatedAt, "")
		if _, err := tx.ExecContext(ctx, `INSERT INTO collections (id, title, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			collection.ID, collection.Title, collection.SortOrder, createdAt, updatedAt); err != nil {
			return fmt.Errorf("restore collection %s: %w", collection.ID, err)
		}
		for index, mangaID := range collection.MangaIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO collection_manga (collection_id, manga_id, sort_order, created_at) VALUES (?, ?, ?, ?)`,
				collection.ID, mangaID, index, createdAt); err != nil {
				return fmt.Errorf("restore collection %s manga: %w", collection.ID, err)
			}
		}
	}
	return nil
}

func insertBackupDiscoverSections(ctx context.Context, tx *sql.Tx, sections []model.BackupDiscoverSection) error {
	for _, section := range sections {
		liveRule, err := jsonValue(section.LiveRule)
		if err != nil {
			return fmt.Errorf("restore discover section %s live rule: %w", section.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO discover_sections (id, title, subtitle, section_type, sort_order, mode, live_rule_json)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			section.ID, section.Title, section.Subtitle, section.Type, section.SortOrder, section.Mode, liveRule); err != nil {
			return fmt.Errorf("restore discover section %s: %w", section.ID, err)
		}
		for index, item := range section.Items {
			metadata, err := rawJSONValue(item.Metadata)
			if err != nil {
				return fmt.Errorf("restore discover item %s metadata: %w", item.ID, err)
			}
			searchQuery, err := jsonValue(item.SearchQuery)
			if err != nil {
				return fmt.Errorf("restore discover item %s search query: %w", item.ID, err)
			}
			publishDate, _ := parseBackupOptionalTime(item.PublishDate, "")
			if _, err := tx.ExecContext(ctx, `
INSERT INTO discover_items (
  id, section_id, item_type, sort_order, manga_id, chapter_id, image_url,
  title, subtitle, supertitle, name, publish_date, content_rating,
  metadata_json, search_query_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.ID, section.ID, item.Type, index, nullString(item.MangaID), nullString(item.ChapterID),
				item.ImageURL, item.Title, item.Subtitle, item.Supertitle, item.Name, nullableUnix(publishDate),
				backupNullableStringPtr(item.ContentRating), metadata, searchQuery); err != nil {
				return fmt.Errorf("restore discover item %s: %w", item.ID, err)
			}
		}
	}
	return nil
}

func parseBackupTime(raw string, field string) (int64, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		if field == "" {
			field = "timestamp"
		}
		return 0, fmt.Errorf("%w: invalid %s", ErrValidation, field)
	}
	return parsed.UTC().Unix(), nil
}

func parseBackupOptionalTime(raw *string, field string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		if field == "" {
			field = "timestamp"
		}
		return nil, fmt.Errorf("%w: invalid %s", ErrValidation, field)
	}
	utc := parsed.UTC()
	return &utc, nil
}

func nullableUnix(value *time.Time) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value.Unix(), Valid: true}
}

func backupNullableStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullString(*value)
}

func nullableOriginMode(origin model.ChapterOrigin) sql.NullString {
	if origin.Provider == nil || strings.TrimSpace(*origin.Provider) == "" {
		return sql.NullString{}
	}
	return nullString(origin.Mode)
}

func jsonValue(value any) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflected.IsNil() {
			return sql.NullString{}, nil
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(raw), Valid: true}, nil
}

func rawJSONValue(raw json.RawMessage) (sql.NullString, error) {
	if raw == nil {
		return sql.NullString{}, nil
	}
	if !json.Valid(raw) {
		return sql.NullString{}, fmt.Errorf("%w: invalid JSON", ErrValidation)
	}
	return sql.NullString{String: string(raw), Valid: true}, nil
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
