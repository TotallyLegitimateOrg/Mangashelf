package store

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/importer"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/services"

	"github.com/google/uuid"
)

type chapterProvenance struct {
	Provider     string
	Mode         string
	SourceID     string
	SourceStatus string
	ChapterKey   string
}

type chapterImportStats struct {
	Inserted  int
	Updated   int
	Unchanged int
	Skipped   int
}

type ArchiveUploadProgress struct {
	Phase    string
	Message  string
	Current  int
	Total    int
	FileName string
}

const (
	ArchiveUploadPhaseExtracting = "extracting_archive"
	ArchiveUploadPhaseUploading  = "uploading_images"
	ArchiveUploadPhaseCreating   = "creating_chapter"
)

var uploadArchivePage = services.UploadToCatbox

func (s *Store) ListChapterSources(ctx context.Context, mangaID string) ([]model.ChapterSource, error) {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return nil, err
	}
	sources, err := s.refreshChapterSources(ctx, mangaID)
	if err != nil {
		return nil, err
	}
	return sources, nil
}

func (s *Store) ListChapterSourceSyncLogs(ctx context.Context, mangaID string) ([]model.ChapterSourceSyncLog, error) {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListChapterSourceSyncLogs(ctx, gen.ListChapterSourceSyncLogsParams{
		MangaID: mangaID,
		Limit:   100,
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.ChapterSourceSyncLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, syncLogFromRow(row))
	}
	return result, nil
}

func (s *Store) ClearChapterSourceSyncLogs(ctx context.Context, mangaID string, sourceID string) error {
	source, err := s.getChapterSource(ctx, sourceID)
	if err != nil {
		return err
	}
	if source.MangaID != mangaID {
		return ErrNotFound
	}
	return s.queries.DeleteChapterSourceSyncLogs(ctx, gen.DeleteChapterSourceSyncLogsParams{
		MangaID:  mangaID,
		SourceID: sourceID,
	})
}

func (s *Store) CreateChapterImport(ctx context.Context, mangaID string, payload model.ChapterImportPayload) (*model.ChapterImportResult, error) {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return nil, err
	}
	payload = model.NormalizeChapterImportPayload(payload)

	provider, canonicalConfig, err := resolveImportProvider(payload.Provider, payload.Config)
	if err != nil {
		return nil, err
	}

	if payload.Mode == "" {
		payload.Mode = "proxy"
	}
	switch payload.Mode {
	case "proxy", "sync", "import_once":
	default:
		return nil, fmt.Errorf("%w: mode must be 'proxy', 'sync', or 'import_once'", ErrValidation)
	}

	if payload.SyncIntervalMinutes <= 0 {
		payload.SyncIntervalMinutes = 60
	}

	normalized, err := provider.FetchSource(ctx, canonicalConfig)
	if err != nil {
		return nil, err
	}
	normalized.Config = canonicalConfig

	switch payload.Mode {
	case "proxy":
		source, err := s.createChapterSource(ctx, mangaID, normalized, payload.Mode, payload.SyncIntervalMinutes)
		if err != nil {
			return nil, err
		}
		return &model.ChapterImportResult{
			Mode:           payload.Mode,
			InsertedCount:  0,
			UpdatedCount:   0,
			UnchangedCount: 0,
			SkippedCount:   0,
			Source:         source,
			Chapters:       nil,
		}, nil
	case "sync":
		return s.createAndSyncChapterSource(ctx, mangaID, normalized, payload.SyncIntervalMinutes)
	default:
		return s.importChaptersOnce(ctx, mangaID, normalized)
	}
}

func (s *Store) createChapterSource(ctx context.Context, mangaID string, normalized *importer.NormalizedSource, mode string, syncIntervalMinutes int) (*model.ChapterSource, error) {
	params, err := buildChapterSourceParams(ctx, s.queries, mangaID, normalized, mode, syncIntervalMinutes, false)
	if err != nil {
		return nil, err
	}
	if err := s.queries.CreateChapterSource(ctx, params); err != nil {
		return nil, err
	}
	return s.getChapterSource(ctx, params.ID)
}

func (s *Store) createAndSyncChapterSource(ctx context.Context, mangaID string, normalized *importer.NormalizedSource, syncIntervalMinutes int) (*model.ChapterImportResult, error) {
	params, err := buildChapterSourceParams(ctx, s.queries, mangaID, normalized, "sync", syncIntervalMinutes, true)
	if err != nil {
		return nil, err
	}

	var importedIDs []string
	var stats chapterImportStats
	if err := s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.CreateChapterSource(ctx, params); err != nil {
			return err
		}
		source := sourceFromCreateParams(params)
		var importErr error
		importedIDs, stats, importErr = s.importNormalizedSourceWithQueries(ctx, q, mangaID, normalized, "sync", &source)
		if importErr != nil {
			return importErr
		}
		return q.CreateChapterSourceSyncLog(ctx, buildChapterSourceSyncLogParams(params.ID, mangaID, "success", stats, "", nowUnix()))
	}); err != nil {
		return nil, err
	}

	source, err := s.getChapterSource(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	chapters, err := s.loadStoredChapterDetails(ctx, importedIDs)
	if err != nil {
		return nil, err
	}
	return &model.ChapterImportResult{
		Mode:           "sync",
		InsertedCount:  stats.Inserted,
		UpdatedCount:   stats.Updated,
		UnchangedCount: stats.Unchanged,
		SkippedCount:   stats.Skipped,
		Source:         source,
		Chapters:       chapters,
	}, nil
}

func (s *Store) importChaptersOnce(ctx context.Context, mangaID string, normalized *importer.NormalizedSource) (*model.ChapterImportResult, error) {
	var importedIDs []string
	var stats chapterImportStats
	if err := s.withTx(ctx, func(q *gen.Queries) error {
		var importErr error
		importedIDs, stats, importErr = s.importNormalizedSourceWithQueries(ctx, q, mangaID, normalized, "import_once", nil)
		return importErr
	}); err != nil {
		return nil, err
	}

	chapters, err := s.loadStoredChapterDetails(ctx, importedIDs)
	if err != nil {
		return nil, err
	}
	return &model.ChapterImportResult{
		Mode:           "import_once",
		InsertedCount:  stats.Inserted,
		UpdatedCount:   stats.Updated,
		UnchangedCount: stats.Unchanged,
		SkippedCount:   stats.Skipped,
		Source:         nil,
		Chapters:       chapters,
	}, nil
}

func (s *Store) UnlinkChapterSource(ctx context.Context, mangaID string, sourceID string) error {
	source, err := s.getChapterSource(ctx, sourceID)
	if err != nil {
		return err
	}
	if source.MangaID != mangaID {
		return ErrNotFound
	}
	return s.queries.DeleteChapterSource(ctx, sourceID)
}

func (s *Store) SyncChapterSource(ctx context.Context, mangaID string, sourceID string) (*model.ChapterSource, chapterImportStats, error) {
	source, err := s.getChapterSource(ctx, sourceID)
	if err != nil {
		return nil, chapterImportStats{}, err
	}
	if source.MangaID != mangaID {
		return nil, chapterImportStats{}, ErrNotFound
	}
	if source.Mode != "sync" {
		return nil, chapterImportStats{}, fmt.Errorf("%w: only sync sources can be synced", ErrValidation)
	}

	provider, err := importer.ResolveProvider(source.Provider)
	if err != nil {
		return nil, chapterImportStats{}, err
	}
	normalized, err := provider.FetchSource(ctx, source.Config)
	now := nowUnix()
	if err != nil {
		s.createChapterSourceSyncLog(ctx, sourceID, mangaID, "error", chapterImportStats{}, err.Error(), now)
		_ = s.queries.UpdateChapterSourceSynced(ctx, gen.UpdateChapterSourceSyncedParams{
			LastSyncedAt:         sql.NullInt64{Int64: now, Valid: true},
			LastSeenChapterCount: sql.NullInt64{},
			Status:               "error",
			LastError:            err.Error(),
			UpdatedAt:            now,
			ID:                   sourceID,
		})
		return nil, chapterImportStats{}, err
	}
	liveSource := *source
	liveSource.Status = "ready"

	var stats chapterImportStats
	if err := s.withTx(ctx, func(q *gen.Queries) error {
		var importErr error
		_, stats, importErr = s.importNormalizedSourceWithQueries(ctx, q, mangaID, normalized, "sync", &liveSource)
		if importErr != nil {
			return importErr
		}
		if err := q.UpdateChapterSourceSynced(ctx, gen.UpdateChapterSourceSyncedParams{
			LastSyncedAt:         sql.NullInt64{Int64: now, Valid: true},
			LastSeenChapterCount: sql.NullInt64{Int64: int64(len(normalized.Chapters)), Valid: true},
			Status:               "ready",
			LastError:            "",
			UpdatedAt:            now,
			ID:                   sourceID,
		}); err != nil {
			return err
		}
		if err := q.CreateChapterSourceSyncLog(ctx, buildChapterSourceSyncLogParams(sourceID, mangaID, "success", stats, "", now)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.createChapterSourceSyncLog(ctx, sourceID, mangaID, "error", chapterImportStats{}, err.Error(), now)
		_ = s.queries.UpdateChapterSourceSynced(ctx, gen.UpdateChapterSourceSyncedParams{
			LastSyncedAt:         sql.NullInt64{Int64: now, Valid: true},
			LastSeenChapterCount: sql.NullInt64{},
			Status:               "error",
			LastError:            err.Error(),
			UpdatedAt:            now,
			ID:                   sourceID,
		})
		return nil, chapterImportStats{}, err
	}

	updated, err := s.getChapterSource(ctx, sourceID)
	if err != nil {
		return nil, chapterImportStats{}, err
	}
	return updated, stats, nil
}

func (s *Store) RunSyncDueSources(ctx context.Context) {
	now := nowUnix()
	rows, err := s.queries.ListSyncDueSources(ctx, sql.NullInt64{Int64: now, Valid: true})
	if err != nil {
		s.log.Error("sync: failed to list due sources", "error", err)
		return
	}
	for _, row := range rows {
		_, _, syncErr := s.SyncChapterSource(ctx, row.MangaID, row.ID)
		if syncErr != nil {
			s.log.Error("sync: failed to sync source", "sourceId", row.ID, "error", syncErr)
		} else {
			s.log.Info("sync: synced source", "sourceId", row.ID, "mangaId", row.MangaID)
		}
	}
}

func (s *Store) ListChapters(ctx context.Context, mangaID string) ([]model.ChapterListItem, error) {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return nil, err
	}

	localChapters, err := s.listStoredChapters(ctx, mangaID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(localChapters))
	for _, chapter := range localChapters {
		seen[importer.CreateChapterIdentityKey(chapter.ChapNum, chapter.LangCode, chapter.Version).Key] = struct{}{}
	}

	sources, err := s.refreshChapterSources(ctx, mangaID)
	if err != nil {
		return nil, err
	}

	result := make([]model.ChapterListItem, 0, len(localChapters))
	result = append(result, localChapters...)
	for _, source := range sources {
		if source.Mode != "proxy" || source.Status == "error" {
			continue
		}
		provider, err := importer.ResolveProvider(source.Provider)
		if err != nil {
			continue
		}
		normalized, err := provider.FetchSource(ctx, source.Config)
		if err != nil {
			continue
		}
		for _, chapter := range normalized.Chapters {
			key := chapter.Identity.Key
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, s.proxyChapter(mangaID, source, chapter).ChapterListItem)
		}
	}

	sortChapters(result)
	return result, nil
}

func (s *Store) GetChapter(ctx context.Context, mangaID string, chapterID string) (*model.ChapterDetail, error) {
	if providerName, sourceID, chapterKey, ok := importer.ParseProxyChapterID(chapterID); ok {
		source, err := s.getChapterSource(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		if source.MangaID != mangaID {
			return nil, ErrNotFound
		}
		if source.Mode != "proxy" {
			return nil, ErrNotFound
		}
		if source.Provider != providerName {
			return nil, ErrNotFound
		}

		localChapters, err := s.listStoredChapters(ctx, mangaID)
		if err != nil {
			return nil, err
		}
		for _, chapter := range localChapters {
			if importer.CreateChapterIdentityKey(chapter.ChapNum, chapter.LangCode, chapter.Version).Key == chapterKey {
				return nil, ErrNotFound
			}
		}

		provider, err := importer.ResolveProvider(source.Provider)
		if err != nil {
			return nil, err
		}
		normalized, err := provider.FetchSource(ctx, source.Config)
		if err != nil {
			if updateErr := s.updateChapterSourceState(ctx, sourceID, "error", err.Error(), nil); updateErr != nil {
				return nil, updateErr
			}
			return nil, err
		}
		count := len(normalized.Chapters)
		_ = s.updateChapterSourceState(ctx, sourceID, "ready", "", &count)
		for _, chapter := range normalized.Chapters {
			if chapter.Identity.Key == chapterKey {
				detail := s.proxyChapter(mangaID, *source, chapter)
				return &detail, nil
			}
		}
		return nil, ErrNotFound
	}

	chapter, err := s.getStoredChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if chapter.MangaID != mangaID {
		return nil, ErrNotFound
	}
	return chapter, nil
}

func (s *Store) CreateChapter(ctx context.Context, mangaID string, payload model.ChapterPayload) (*model.ChapterDetail, error) {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return nil, err
	}
	payload = model.NormalizeChapterPayload(payload)
	if payload.ChapNum == 0 {
		return nil, fmt.Errorf("%w: chapter number is required", ErrValidation)
	}
	return s.writeChapter(ctx, "", mangaID, payload, nil)
}

func (s *Store) UpdateChapter(ctx context.Context, mangaID string, chapterID string, payload model.ChapterPayload) (*model.ChapterDetail, error) {
	if _, _, _, ok := importer.ParseProxyChapterID(chapterID); ok {
		return nil, ErrForbidden
	}
	existing, err := s.getStoredChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if existing.MangaID != mangaID {
		return nil, ErrNotFound
	}
	payload = model.NormalizeChapterPayload(payload)
	return s.writeChapter(ctx, chapterID, mangaID, payload, provenanceFromOrigin(existing.Origin))
}

func (s *Store) DeleteChapter(ctx context.Context, mangaID string, chapterID string) error {
	if _, _, _, ok := importer.ParseProxyChapterID(chapterID); ok {
		return ErrForbidden
	}
	chapter, err := s.getStoredChapter(ctx, chapterID)
	if err != nil {
		return err
	}
	if chapter.MangaID != mangaID {
		return ErrNotFound
	}

	return s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.DeleteDiscoverItemsByChapterID(ctx, nullString(chapterID)); err != nil {
			return err
		}
		if err := q.DeleteChapter(ctx, chapterID); err != nil {
			return err
		}
		return q.TouchMangaUpdatedAt(ctx, gen.TouchMangaUpdatedAtParams{
			UpdatedAt: nowUnix(),
			ID:        mangaID,
		})
	})
}

func (s *Store) ReorderChapters(ctx context.Context, mangaID string, order []string) error {
	if _, err := s.loadManga(ctx, mangaID); err != nil {
		return err
	}
	if len(order) == 0 {
		return s.queries.ClearChapterSortingIndices(ctx, mangaID)
	}
	return s.withTx(ctx, func(q *gen.Queries) error {
		for index, chapterID := range order {
			sortingIndex := float64(index)
			if err := q.UpdateChapterSortingIndex(ctx, gen.UpdateChapterSortingIndexParams{
				SortingIndex: sql.NullFloat64{Float64: sortingIndex, Valid: true},
				ID:           chapterID,
				MangaID:      mangaID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) CreateChapterFromArchive(ctx context.Context, mangaID string, header *multipart.FileHeader, payload model.ChapterPayload) (*model.ChapterDetail, error) {
	return s.CreateChapterFromArchiveWithProgress(ctx, mangaID, header, payload, nil)
}

func (s *Store) CreateChapterFromArchiveWithProgress(ctx context.Context, mangaID string, header *multipart.FileHeader, payload model.ChapterPayload, onProgress func(ArchiveUploadProgress)) (*model.ChapterDetail, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	archiveFiles := make([]*zip.File, 0)
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || !isSupportedArchiveImage(entry.Name) {
			continue
		}
		archiveFiles = append(archiveFiles, entry)
	}
	reportArchiveUploadProgress(onProgress, ArchiveUploadProgress{
		Phase:   ArchiveUploadPhaseExtracting,
		Message: "Extracting archive",
		Total:   len(archiveFiles),
	})

	entries := make([]struct {
		Name string
		Data []byte
	}, 0)
	for i, entry := range archiveFiles {
		content, err := func() ([]byte, error) {
			rc, err := entry.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}()
		if err != nil {
			return nil, err
		}
		entries = append(entries, struct {
			Name string
			Data []byte
		}{Name: entry.Name, Data: content})
		reportArchiveUploadProgress(onProgress, ArchiveUploadProgress{
			Phase:    ArchiveUploadPhaseExtracting,
			Message:  "Extracting archive",
			Current:  i + 1,
			Total:    len(archiveFiles),
			FileName: filepath.Base(entry.Name),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: no images found in the archive", ErrValidation)
	}

	pages := make([]string, 0, len(entries))
	for i, entry := range entries {
		reportArchiveUploadProgress(onProgress, ArchiveUploadProgress{
			Phase:    ArchiveUploadPhaseUploading,
			Message:  "Uploading extracted images",
			Current:  i + 1,
			Total:    len(entries),
			FileName: filepath.Base(entry.Name),
		})
		url, err := uploadArchivePage(ctx, filepath.Base(entry.Name), entry.Data)
		if err != nil {
			return nil, err
		}
		pages = append(pages, url)
	}
	reportArchiveUploadProgress(onProgress, ArchiveUploadProgress{
		Phase:   ArchiveUploadPhaseCreating,
		Message: "Saving chapter",
	})
	payload.Pages = pages
	return s.CreateChapter(ctx, mangaID, payload)
}

func isSupportedArchiveImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif":
		return true
	default:
		return false
	}
}

func reportArchiveUploadProgress(onProgress func(ArchiveUploadProgress), progress ArchiveUploadProgress) {
	if onProgress != nil {
		onProgress(progress)
	}
}

func (s *Store) writeChapter(ctx context.Context, chapterID string, mangaID string, payload model.ChapterPayload, provenance *chapterProvenance) (*model.ChapterDetail, error) {
	now := nowUnix()
	err := s.withTx(ctx, func(q *gen.Queries) error {
		if err := ensureMangaExists(ctx, q, mangaID); err != nil {
			return err
		}
		var writeErr error
		chapterID, writeErr = s.writeChapterWithQueries(ctx, q, chapterID, mangaID, payload, provenance, now, true)
		return writeErr
	})
	if err != nil {
		return nil, err
	}
	return s.getStoredChapter(ctx, chapterID)
}

func ensureMangaExists(ctx context.Context, q *gen.Queries, mangaID string) error {
	if _, err := q.GetMangaByID(ctx, mangaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Store) importNormalizedSourceWithQueries(ctx context.Context, q *gen.Queries, mangaID string, normalized *importer.NormalizedSource, mode string, source *model.ChapterSource) ([]string, chapterImportStats, error) {
	if err := ensureMangaExists(ctx, q, mangaID); err != nil {
		return nil, chapterImportStats{}, err
	}

	existing, err := listStoredChaptersByIdentityWithQueries(ctx, q, mangaID)
	if err != nil {
		return nil, chapterImportStats{}, err
	}

	now := nowUnix()
	importedIDs := make([]string, 0, len(normalized.Chapters))
	var stats chapterImportStats
	for _, chapter := range normalized.Chapters {
		row, exists := existing[chapter.Identity.Key]
		provenance := provenanceForImportedChapter(normalized.Provider, mode, source, chapter.Identity.Key)
		chapterPayload := model.ChapterPayload{
			LangCode:       chapter.LangCode,
			ChapNum:        chapter.ChapNum,
			Title:          chapter.Title,
			Version:        chapter.Version,
			Volume:         chapter.Volume,
			PublishDate:    stringPtr(chapter.PublishDate),
			CreationDate:   stringPtr(chapter.CreationDate),
			SortingIndex:   nil,
			AdditionalInfo: importedChapterAdditionalInfo(normalized, source),
			Pages:          chapter.Pages,
		}

		if exists {
			if !canUpdateImportedChapter(row, source, chapter.Identity.Key) {
				stats.Skipped++
				continue
			}
			changed, err := storedChapterDiffersFromSource(ctx, q, row, chapterPayload, provenance)
			if err != nil {
				return nil, chapterImportStats{}, err
			}
			if !changed {
				stats.Unchanged++
				continue
			}
		}

		chapterID, err := s.writeChapterWithQueries(ctx, q, storedChapterID(row, exists), mangaID, chapterPayload, provenance, now, false)
		if err != nil {
			return nil, chapterImportStats{}, err
		}
		if exists {
			stats.Updated++
		} else {
			stats.Inserted++
		}
		importedIDs = append(importedIDs, chapterID)
	}

	if stats.Inserted+stats.Updated > 0 {
		if err := q.TouchMangaUpdatedAt(ctx, gen.TouchMangaUpdatedAtParams{
			UpdatedAt: now,
			ID:        mangaID,
		}); err != nil {
			return nil, chapterImportStats{}, err
		}
	}
	return importedIDs, stats, nil
}

func storedChapterDiffersFromSource(ctx context.Context, q *gen.Queries, row gen.ListStoredChaptersRow, payload model.ChapterPayload, provenance *chapterProvenance) (bool, error) {
	publishDate, err := parseTimePointer(payload.PublishDate)
	if err != nil {
		return false, err
	}
	creationDate, err := parseTimePointer(payload.CreationDate)
	if err != nil {
		return false, err
	}

	if row.LangCode != payload.LangCode ||
		row.ChapNum != payload.ChapNum ||
		row.Title != payload.Title ||
		row.Version != payload.Version ||
		!nullFloatsEqual(row.Volume, nullFloat(payload.Volume)) ||
		!nullIntsEqual(row.PublishDate, nullIntFromTime(publishDate)) ||
		!nullIntsEqual(row.CreationDate, nullIntFromTime(creationDate)) ||
		!nullFloatsEqual(row.SortingIndex, nullFloat(payload.SortingIndex)) ||
		!nullStringsEqual(row.OriginProvider, nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.Provider }))) ||
		!nullStringsEqual(row.OriginMode, nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.Mode }))) ||
		!nullStringsEqual(row.OriginSourceID, nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.SourceID }))) ||
		!nullStringsEqual(row.OriginSourceStatus, nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.SourceStatus }))) ||
		!nullStringsEqual(row.OriginChapterKey, nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.ChapterKey }))) {
		return true, nil
	}

	infoEntries, err := q.ListChapterInfoEntries(ctx, row.ID)
	if err != nil {
		return false, err
	}
	if len(infoEntries) != len(payload.AdditionalInfo) {
		return true, nil
	}
	for index, entry := range infoEntries {
		if entry.InfoKey != payload.AdditionalInfo[index].Key || entry.InfoValue != payload.AdditionalInfo[index].Value {
			return true, nil
		}
	}

	pages, err := q.ListChapterPages(ctx, row.ID)
	if err != nil {
		return false, err
	}
	if len(pages) != len(payload.Pages) {
		return true, nil
	}
	for index, page := range pages {
		if page.ImageUrl != payload.Pages[index] {
			return true, nil
		}
	}
	return false, nil
}

func nullFloatsEqual(a sql.NullFloat64, b sql.NullFloat64) bool {
	return a.Valid == b.Valid && (!a.Valid || a.Float64 == b.Float64)
}

func nullIntsEqual(a sql.NullInt64, b sql.NullInt64) bool {
	return a.Valid == b.Valid && (!a.Valid || a.Int64 == b.Int64)
}

func nullStringsEqual(a sql.NullString, b sql.NullString) bool {
	return a.Valid == b.Valid && (!a.Valid || a.String == b.String)
}

func importedChapterAdditionalInfo(normalized *importer.NormalizedSource, source *model.ChapterSource) []model.InfoEntry {
	return []model.InfoEntry{
		{Key: "Provider", Value: normalized.Provider},
	}
}

func listStoredChaptersByIdentityWithQueries(ctx context.Context, q *gen.Queries, mangaID string) (map[string]gen.ListStoredChaptersRow, error) {
	rows, err := q.ListStoredChapters(ctx, mangaID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]gen.ListStoredChaptersRow, len(rows))
	for _, row := range rows {
		seen[importer.CreateChapterIdentityKey(row.ChapNum, row.LangCode, row.Version).Key] = row
	}
	return seen, nil
}

func (s *Store) writeChapterWithQueries(ctx context.Context, q *gen.Queries, chapterID string, mangaID string, payload model.ChapterPayload, provenance *chapterProvenance, now int64, touchManga bool) (string, error) {
	if payload.ChapNum == 0 {
		return "", fmt.Errorf("%w: chapter number is required", ErrValidation)
	}
	publishDate, err := parseTimePointer(payload.PublishDate)
	if err != nil {
		return "", err
	}
	creationDate, err := parseTimePointer(payload.CreationDate)
	if err != nil {
		return "", err
	}

	if chapterID == "" {
		chapterID = uuid.NewString()
	}

	params := gen.CreateChapterParams{
		ID:                 chapterID,
		MangaID:            mangaID,
		LangCode:           payload.LangCode,
		ChapNum:            payload.ChapNum,
		Title:              payload.Title,
		Version:            payload.Version,
		Volume:             nullFloat(payload.Volume),
		PublishDate:        nullIntFromTime(publishDate),
		CreationDate:       nullIntFromTime(creationDate),
		SortingIndex:       nullFloat(payload.SortingIndex),
		OriginProvider:     nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.Provider })),
		OriginMode:         nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.Mode })),
		OriginSourceID:     nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.SourceID })),
		OriginSourceStatus: nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.SourceStatus })),
		OriginChapterKey:   nullString(provenanceValue(provenance, func(p *chapterProvenance) string { return p.ChapterKey })),
		LastUpdated:        now,
	}

	if _, err := q.GetStoredChapterByID(ctx, chapterID); err == nil {
		if err := q.UpdateChapter(ctx, gen.UpdateChapterParams{
			LangCode:           params.LangCode,
			ChapNum:            params.ChapNum,
			Title:              params.Title,
			Version:            params.Version,
			Volume:             params.Volume,
			PublishDate:        params.PublishDate,
			CreationDate:       params.CreationDate,
			SortingIndex:       params.SortingIndex,
			OriginProvider:     params.OriginProvider,
			OriginMode:         params.OriginMode,
			OriginSourceID:     params.OriginSourceID,
			OriginSourceStatus: params.OriginSourceStatus,
			OriginChapterKey:   params.OriginChapterKey,
			LastUpdated:        params.LastUpdated,
			ID:                 chapterID,
		}); err != nil {
			return "", err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		if err := q.CreateChapter(ctx, params); err != nil {
			return "", err
		}
	} else {
		return "", err
	}

	if err := q.DeleteChapterInfoEntries(ctx, chapterID); err != nil {
		return "", err
	}
	if err := q.DeleteChapterPages(ctx, chapterID); err != nil {
		return "", err
	}
	for index, entry := range payload.AdditionalInfo {
		if err := q.InsertChapterInfoEntry(ctx, gen.InsertChapterInfoEntryParams{
			ID:        uuid.NewString(),
			ChapterID: chapterID,
			InfoKey:   entry.Key,
			InfoValue: entry.Value,
			SortOrder: int64(index),
		}); err != nil {
			return "", err
		}
	}
	for index, page := range payload.Pages {
		if err := q.InsertChapterPage(ctx, gen.InsertChapterPageParams{
			ID:        uuid.NewString(),
			ChapterID: chapterID,
			PageNum:   int64(index + 1),
			ImageUrl:  page,
		}); err != nil {
			return "", err
		}
	}
	if touchManga {
		if err := q.TouchMangaUpdatedAt(ctx, gen.TouchMangaUpdatedAtParams{
			UpdatedAt: now,
			ID:        mangaID,
		}); err != nil {
			return "", err
		}
	}
	return chapterID, nil
}

func (s *Store) listStoredChapters(ctx context.Context, mangaID string) ([]model.ChapterListItem, error) {
	rows, err := s.queries.ListStoredChapters(ctx, mangaID)
	if err != nil {
		return nil, err
	}
	result := make([]model.ChapterListItem, 0, len(rows))
	for _, row := range rows {
		infoEntries, err := s.queries.ListChapterInfoEntries(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		info := make([]model.InfoEntry, 0, len(infoEntries))
		for _, entry := range infoEntries {
			info = append(info, model.InfoEntry{Key: entry.InfoKey, Value: entry.InfoValue})
		}
		result = append(result, model.ChapterListItem{
			ID:             row.ID,
			MangaID:        row.MangaID,
			LangCode:       row.LangCode,
			ChapNum:        row.ChapNum,
			Title:          row.Title,
			Version:        row.Version,
			Volume:         floatPtrFromNull(row.Volume),
			PublishDate:    isoStringPtrFromNull(row.PublishDate),
			CreationDate:   isoStringPtrFromNull(row.CreationDate),
			SortingIndex:   floatPtrFromNull(row.SortingIndex),
			AdditionalInfo: info,
			PageCount:      int(row.PageCount),
			LastUpdated:    isoString(row.LastUpdated),
			Origin:         storedOrigin(row.OriginProvider, row.OriginMode, row.OriginSourceID, row.OriginSourceStatus, row.OriginChapterKey),
		})
	}
	sortChapters(result)
	return result, nil
}

func (s *Store) getStoredChapter(ctx context.Context, chapterID string) (*model.ChapterDetail, error) {
	row, err := s.queries.GetStoredChapterByID(ctx, chapterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	infoEntries, err := s.queries.ListChapterInfoEntries(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	pages, err := s.queries.ListChapterPages(ctx, chapterID)
	if err != nil {
		return nil, err
	}

	info := make([]model.InfoEntry, 0, len(infoEntries))
	for _, entry := range infoEntries {
		info = append(info, model.InfoEntry{Key: entry.InfoKey, Value: entry.InfoValue})
	}
	pageURLs := make([]string, 0, len(pages))
	for _, page := range pages {
		pageURLs = append(pageURLs, page.ImageUrl)
	}

	return &model.ChapterDetail{
		ChapterListItem: model.ChapterListItem{
			ID:             row.ID,
			MangaID:        row.MangaID,
			LangCode:       row.LangCode,
			ChapNum:        row.ChapNum,
			Title:          row.Title,
			Version:        row.Version,
			Volume:         floatPtrFromNull(row.Volume),
			PublishDate:    isoStringPtrFromNull(row.PublishDate),
			CreationDate:   isoStringPtrFromNull(row.CreationDate),
			SortingIndex:   floatPtrFromNull(row.SortingIndex),
			AdditionalInfo: info,
			PageCount:      int(row.PageCount),
			LastUpdated:    isoString(row.LastUpdated),
			Origin:         storedOrigin(row.OriginProvider, row.OriginMode, row.OriginSourceID, row.OriginSourceStatus, row.OriginChapterKey),
		},
		Pages: pageURLs,
	}, nil
}

func (s *Store) getChapterSource(ctx context.Context, sourceID string) (*model.ChapterSource, error) {
	source, err := s.queries.GetChapterSourceByID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &model.ChapterSource{
		ID:                   source.ID,
		MangaID:              source.MangaID,
		Provider:             source.Provider,
		Mode:                 source.Mode,
		Config:               []byte(source.ConfigJson),
		Status:               source.Status,
		LastError:            source.LastError,
		LastSeenChapterCount: intPtrFromNull(source.LastSeenChapterCount),
		SyncIntervalMinutes:  int(source.SyncIntervalMinutes),
		LastSyncedAt:         isoStringPtrFromNull(source.LastSyncedAt),
		CreatedAt:            isoString(source.CreatedAt),
		UpdatedAt:            isoString(source.UpdatedAt),
	}, nil
}

func (s *Store) refreshChapterSources(ctx context.Context, mangaID string) ([]model.ChapterSource, error) {
	rows, err := s.queries.ListChapterSources(ctx, mangaID)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.Mode == "sync" {
			continue // sync sources don't live-refresh
		}
		source := sourceFromRow(row)
		provider, err := importer.ResolveProvider(source.Provider)
		if err != nil {
			_ = s.updateChapterSourceState(ctx, source.ID, "error", err.Error(), nil)
			continue
		}
		normalized, err := provider.FetchSource(ctx, source.Config)
		if err != nil {
			_ = s.updateChapterSourceState(ctx, source.ID, "error", err.Error(), nil)
			continue
		}
		count := len(normalized.Chapters)
		_ = s.updateChapterSourceState(ctx, source.ID, "ready", "", &count)
	}

	refreshed, err := s.queries.ListChapterSources(ctx, mangaID)
	if err != nil {
		return nil, err
	}
	result := make([]model.ChapterSource, 0, len(refreshed))
	for _, row := range refreshed {
		result = append(result, sourceFromRow(row))
	}
	return result, nil
}

func (s *Store) updateChapterSourceState(ctx context.Context, sourceID string, status string, lastError string, count *int) error {
	params := gen.UpdateChapterSourceParams{
		Status:               status,
		LastError:            lastError,
		LastSeenChapterCount: sql.NullInt64{},
		UpdatedAt:            nowUnix(),
		ID:                   sourceID,
	}
	if count != nil {
		params.LastSeenChapterCount = sql.NullInt64{Int64: int64(*count), Valid: true}
	}
	return s.queries.UpdateChapterSource(ctx, params)
}

func (s *Store) proxyChapter(mangaID string, source model.ChapterSource, chapter importer.NormalizedChapter) model.ChapterDetail {
	return model.ChapterDetail{
		ChapterListItem: model.ChapterListItem{
			ID:             importer.CreateProxyChapterID(source.Provider, source.ID, chapter.Identity),
			MangaID:        mangaID,
			LangCode:       chapter.LangCode,
			ChapNum:        chapter.ChapNum,
			Title:          chapter.Title,
			Version:        chapter.Version,
			Volume:         chapter.Volume,
			PublishDate:    stringPtr(chapter.PublishDate),
			CreationDate:   stringPtr(chapter.CreationDate),
			SortingIndex:   nil,
			AdditionalInfo: nil,
			PageCount:      len(chapter.Pages),
			LastUpdated:    chapter.LastUpdated.UTC().Format(time.RFC3339),
			Origin:         proxyChapterOrigin(source, chapter.Identity.Key),
		},
		Pages: chapter.Pages,
	}
}

func buildChapterSourceParams(ctx context.Context, q *gen.Queries, mangaID string, normalized *importer.NormalizedSource, mode string, syncIntervalMinutes int, setLastSynced bool) (gen.CreateChapterSourceParams, error) {
	if mode != "proxy" && mode != "sync" {
		return gen.CreateChapterSourceParams{}, fmt.Errorf("%w: mode must be 'proxy' or 'sync'", ErrValidation)
	}

	existing, err := q.FindChapterSourceByConfig(ctx, gen.FindChapterSourceByConfigParams{
		MangaID:    mangaID,
		Provider:   normalized.Provider,
		ConfigJson: string(normalized.Config),
	})
	if err != nil {
		return gen.CreateChapterSourceParams{}, err
	}
	if len(existing) > 0 {
		return gen.CreateChapterSourceParams{}, ErrConflict
	}

	now := nowUnix()
	params := gen.CreateChapterSourceParams{
		ID:                   uuid.NewString(),
		MangaID:              mangaID,
		Provider:             normalized.Provider,
		Mode:                 mode,
		ConfigJson:           string(normalized.Config),
		Status:               "ready",
		LastError:            "",
		LastSeenChapterCount: sql.NullInt64{Int64: int64(len(normalized.Chapters)), Valid: true},
		SyncIntervalMinutes:  int64(syncIntervalMinutes),
		LastSyncedAt:         sql.NullInt64{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if setLastSynced {
		params.LastSyncedAt = sql.NullInt64{Int64: now, Valid: true}
	}
	return params, nil
}

func sourceFromCreateParams(params gen.CreateChapterSourceParams) model.ChapterSource {
	return model.ChapterSource{
		ID:                   params.ID,
		MangaID:              params.MangaID,
		Provider:             params.Provider,
		Mode:                 params.Mode,
		Config:               []byte(params.ConfigJson),
		Status:               params.Status,
		LastError:            params.LastError,
		LastSeenChapterCount: intPtrFromNull(params.LastSeenChapterCount),
		SyncIntervalMinutes:  int(params.SyncIntervalMinutes),
		LastSyncedAt:         isoStringPtrFromNull(params.LastSyncedAt),
		CreatedAt:            isoString(params.CreatedAt),
		UpdatedAt:            isoString(params.UpdatedAt),
	}
}

func sourceFromRow(row gen.ChapterSource) model.ChapterSource {
	return model.ChapterSource{
		ID:                   row.ID,
		MangaID:              row.MangaID,
		Provider:             row.Provider,
		Mode:                 row.Mode,
		Config:               []byte(row.ConfigJson),
		Status:               row.Status,
		LastError:            row.LastError,
		LastSeenChapterCount: intPtrFromNull(row.LastSeenChapterCount),
		SyncIntervalMinutes:  int(row.SyncIntervalMinutes),
		LastSyncedAt:         isoStringPtrFromNull(row.LastSyncedAt),
		CreatedAt:            isoString(row.CreatedAt),
		UpdatedAt:            isoString(row.UpdatedAt),
	}
}

func syncLogFromRow(row gen.ChapterSourceSyncLog) model.ChapterSourceSyncLog {
	return model.ChapterSourceSyncLog{
		ID:             row.ID,
		SourceID:       row.SourceID,
		MangaID:        row.MangaID,
		Status:         row.Status,
		InsertedCount:  int(row.InsertedCount),
		UpdatedCount:   int(row.UpdatedCount),
		UnchangedCount: int(row.UnchangedCount),
		SkippedCount:   int(row.SkippedCount),
		Error:          row.Error,
		CreatedAt:      isoString(row.CreatedAt),
	}
}

func buildChapterSourceSyncLogParams(sourceID string, mangaID string, status string, stats chapterImportStats, syncErr string, now int64) gen.CreateChapterSourceSyncLogParams {
	return gen.CreateChapterSourceSyncLogParams{
		ID:             uuid.NewString(),
		SourceID:       sourceID,
		MangaID:        mangaID,
		Status:         status,
		InsertedCount:  int64(stats.Inserted),
		UpdatedCount:   int64(stats.Updated),
		UnchangedCount: int64(stats.Unchanged),
		SkippedCount:   int64(stats.Skipped),
		Error:          syncErr,
		CreatedAt:      now,
	}
}

func (s *Store) createChapterSourceSyncLog(ctx context.Context, sourceID string, mangaID string, status string, stats chapterImportStats, syncErr string, now int64) {
	if err := s.queries.CreateChapterSourceSyncLog(ctx, buildChapterSourceSyncLogParams(sourceID, mangaID, status, stats, syncErr, now)); err != nil {
		s.log.Error("sync: failed to persist sync log", "sourceId", sourceID, "error", err)
	}
}

func resolveImportProvider(providerName string, config []byte) (importer.Provider, []byte, error) {
	provider, err := importer.ResolveProvider(providerName)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}
	canonicalConfig, err := provider.ParseConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}
	return provider, canonicalConfig, nil
}

func provenanceForImportedChapter(provider string, mode string, source *model.ChapterSource, chapterKey string) *chapterProvenance {
	provenance := &chapterProvenance{
		Provider:   provider,
		Mode:       mode,
		ChapterKey: chapterKey,
	}
	if source != nil {
		provenance.SourceID = source.ID
		provenance.SourceStatus = source.Status
	}
	return provenance
}

func provenanceFromOrigin(origin model.ChapterOrigin) *chapterProvenance {
	if origin.Provider == nil || *origin.Provider == "" {
		return nil
	}
	return &chapterProvenance{
		Provider:     derefString(origin.Provider),
		Mode:         origin.Mode,
		SourceID:     derefString(origin.SourceID),
		SourceStatus: derefString(origin.SourceStatus),
		ChapterKey:   derefString(origin.ChapterKey),
	}
}

func canUpdateImportedChapter(row gen.ListStoredChaptersRow, source *model.ChapterSource, chapterKey string) bool {
	if source == nil {
		return false
	}
	return row.OriginProvider.Valid &&
		row.OriginProvider.String == source.Provider &&
		row.OriginMode.Valid &&
		row.OriginMode.String == "sync" &&
		row.OriginSourceID.Valid &&
		row.OriginSourceID.String == source.ID &&
		row.OriginChapterKey.Valid &&
		row.OriginChapterKey.String == chapterKey
}

func storedChapterID(row gen.ListStoredChaptersRow, exists bool) string {
	if !exists {
		return ""
	}
	return row.ID
}

func provenanceValue(provenance *chapterProvenance, fn func(*chapterProvenance) string) string {
	if provenance == nil {
		return ""
	}
	return fn(provenance)
}

func storedOrigin(provider sql.NullString, mode sql.NullString, sourceID sql.NullString, sourceStatus sql.NullString, chapterKey sql.NullString) model.ChapterOrigin {
	if !provider.Valid {
		return localChapterOrigin()
	}
	return importedChapterOrigin(
		provider.String,
		firstNonEmpty(mode.String, "import_once"),
		nullStringPointer(sourceID),
		nullStringPointer(sourceStatus),
		nullStringPointer(chapterKey),
	)
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return stringPointerOrNil(value.String)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *Store) loadStoredChapterDetails(ctx context.Context, chapterIDs []string) ([]model.ChapterDetail, error) {
	result := make([]model.ChapterDetail, 0, len(chapterIDs))
	for _, chapterID := range chapterIDs {
		chapter, err := s.getStoredChapter(ctx, chapterID)
		if err != nil {
			return nil, err
		}
		result = append(result, *chapter)
	}
	return result, nil
}

func stringPtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
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
