package store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func TestExportBackupIncludesRestorableDataAndOmitsSensitiveFields(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	user, err := store.CreateInitialUser(ctx, "backup-user", "secret123")
	if err != nil {
		t.Fatalf("CreateInitialUser returned error: %v", err)
	}
	if _, err := store.CreateAPIKey(ctx, user.ID, "backup-key"); err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	first := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{
		PrimaryTitle:    "First",
		SecondaryTitles: []string{"Alt First"},
		Synopsis:        "First synopsis",
		ThumbnailURL:    "https://example.com/first-thumb.jpg",
		BannerURL:       "https://example.com/first-banner.jpg",
		ContentRating:   "SAFE",
		Status:          "Ongoing",
		Artist:          "Artist A",
		Author:          "Author A",
		Rating:          floatPtr(8.4),
		ShareURL:        "https://example.com/first",
		ArtworkURLs:     []string{"https://example.com/first-art.jpg"},
		TagGroups: []model.TagGroup{
			{Title: "Genres", Tags: []model.Tag{{Title: "Action"}}},
		},
		AdditionalInfo: []model.InfoEntry{
			{Key: "Source", Value: "Manual"},
		},
	})
	second := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{
		PrimaryTitle: "Second",
	})

	localChapter := mustCreateChapterForManga(t, store, ctx, first.ID, 1, "Local Chapter", "2024-03-10")

	proxyFixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"7": {"https://example.com/proxy-only.jpg"},
	}))
	if _, err := store.CreateChapterImport(ctx, first.ID, chapterImportPayload("cubari", "proxy", proxyFixture.URL())); err != nil {
		t.Fatalf("CreateChapterImport proxy returned error: %v", err)
	}

	syncFixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"2": {"https://example.com/sync-page.jpg"},
	}))
	syncResult, err := store.CreateChapterImport(ctx, second.ID, chapterImportPayload("cubari", "sync", syncFixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport sync returned error: %v", err)
	}
	if syncResult.Source == nil {
		t.Fatalf("CreateChapterImport sync returned nil source")
	}

	collection, err := store.CreateCollection(ctx, model.CollectionPayload{Title: "Favorites"})
	if err != nil {
		t.Fatalf("CreateCollection returned error: %v", err)
	}
	if err := store.ReplaceCollectionManga(ctx, collection.ID, model.CollectionMangaPayload{
		MangaIDs: []string{second.ID, first.ID},
	}); err != nil {
		t.Fatalf("ReplaceCollectionManga returned error: %v", err)
	}

	manualSection, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Manual",
		Type:  "simpleCarousel",
		Mode:  "manual",
		Items: []model.DiscoverSectionItem{
			{
				Type:     "simpleCarouselItem",
				MangaID:  first.ID,
				ImageURL: "https://example.com/manual.jpg",
				Title:    first.PrimaryTitle,
				Subtitle: "Stored item",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection manual returned error: %v", err)
	}
	liveSection, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Live",
		Type:  "simpleCarousel",
		Mode:  "live",
		LiveRule: &model.DiscoverLiveRule{
			Preset: "title_asc",
			Limit:  3,
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection live returned error: %v", err)
	}

	backup, err := store.ExportBackup(ctx)
	if err != nil {
		t.Fatalf("ExportBackup returned error: %v", err)
	}

	if backup.SchemaVersion != model.BackupSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", backup.SchemaVersion, model.BackupSchemaVersion)
	}
	if len(backup.Manga) != 2 {
		t.Fatalf("manga count = %d, want 2", len(backup.Manga))
	}
	if len(backup.Chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2 stored chapters", len(backup.Chapters))
	}
	if len(backup.Collections) != 1 {
		t.Fatalf("collection count = %d, want 1", len(backup.Collections))
	}
	if len(backup.ChapterSources) != 2 {
		t.Fatalf("chapter source count = %d, want 2", len(backup.ChapterSources))
	}
	if len(backup.DiscoverSections) != 2 {
		t.Fatalf("discover section count = %d, want 2", len(backup.DiscoverSections))
	}

	if backup.Collections[0].Title != "Favorites" {
		t.Fatalf("collection title = %q, want Favorites", backup.Collections[0].Title)
	}
	if got := backup.Collections[0].MangaIDs; len(got) != 2 || got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("collection manga IDs = %#v, want [%q %q]", got, second.ID, first.ID)
	}

	if backup.Chapters[0].ID != localChapter.ID {
		t.Fatalf("first exported chapter = %q, want local chapter %q", backup.Chapters[0].ID, localChapter.ID)
	}
	for _, chapter := range backup.Chapters {
		if chapter.Origin.Mode == "proxy" {
			t.Fatalf("backup should not include proxy-only chapters: %+v", chapter)
		}
	}

	var foundProxySource bool
	for _, source := range backup.ChapterSources {
		if source.Mode == "proxy" {
			foundProxySource = true
		}
		if source.Provider != "cubari" {
			t.Fatalf("chapter source provider = %q, want cubari", source.Provider)
		}
		if source.Config == nil || !json.Valid(source.Config) {
			t.Fatalf("chapter source config = %q, want valid JSON", string(source.Config))
		}
	}
	if !foundProxySource {
		t.Fatalf("expected at least one proxy source in backup")
	}

	sectionsByID := make(map[string]model.BackupDiscoverSection, len(backup.DiscoverSections))
	for _, section := range backup.DiscoverSections {
		sectionsByID[section.ID] = section
	}
	if got := sectionsByID[manualSection.ID]; len(got.Items) != 1 || got.Items[0].Subtitle != "Stored item" || !isUUIDv7(got.Items[0].ID) {
		t.Fatalf("manual discover section = %+v, want stored manual item", got)
	}
	if got := sectionsByID[liveSection.ID]; got.LiveRule == nil || got.LiveRule.Preset != "title_asc" || len(got.Items) != 0 {
		t.Fatalf("live discover section = %+v, want liveRule with no derived items", got)
	}

	body := marshalBackupJSON(t, backup)
	unwanted := []string{
		`"users"`,
		`"apiKeys"`,
		`"passwordHash"`,
		`"keyHash"`,
		`"keyPrefix"`,
		`"syncLogs"`,
		`"lastError"`,
		`"lastSeenChapterCount"`,
		`"lastSyncedAt"`,
	}
	for _, token := range unwanted {
		if strings.Contains(string(body), token) {
			t.Fatalf("backup JSON unexpectedly contains %s", token)
		}
	}
}

func TestExportBackupIsDeterministic(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	manga := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Deterministic"})
	mustCreateChapterForManga(t, store, ctx, manga.ID, 2, "Later", "2024-04-02")
	mustCreateChapterForManga(t, store, ctx, manga.ID, 1, "Earlier", "2024-04-01")

	first, err := store.ExportBackup(ctx)
	if err != nil {
		t.Fatalf("first ExportBackup returned error: %v", err)
	}
	second, err := store.ExportBackup(ctx)
	if err != nil {
		t.Fatalf("second ExportBackup returned error: %v", err)
	}

	firstJSON := marshalBackupJSON(t, first)
	secondJSON := marshalBackupJSON(t, second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("backup JSON differs across repeated exports\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestExportBackupArchiveWritesDiffableTree(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	manga := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Archive"})
	chapter := mustCreateChapterForManga(t, store, ctx, manga.ID, 1, "Chapter", "2024-04-01")

	var first bytes.Buffer
	if err := store.ExportBackupArchive(ctx, &first); err != nil {
		t.Fatalf("ExportBackupArchive first returned error: %v", err)
	}
	var second bytes.Buffer
	if err := store.ExportBackupArchive(ctx, &second); err != nil {
		t.Fatalf("ExportBackupArchive second returned error: %v", err)
	}

	firstFiles := readBackupArchiveFiles(t, first.Bytes())
	secondFiles := readBackupArchiveFiles(t, second.Bytes())
	for _, name := range []string{
		"manifest.json",
		"manga/" + manga.ID + ".json",
		"chapters/" + chapter.ID + ".json",
	} {
		if _, ok := firstFiles[name]; !ok {
			t.Fatalf("backup archive missing %s", name)
		}
	}
	delete(firstFiles, "manifest.json")
	delete(secondFiles, "manifest.json")
	if !mapsEqual(firstFiles, secondFiles) {
		t.Fatalf("entity files differ across repeated archive exports")
	}
}

func TestRestoreBackupRoundTripsExportedData(t *testing.T) {
	ctx := context.Background()
	source := newTestStore(t)

	user, err := source.CreateInitialUser(ctx, "source-user", "secret123")
	if err != nil {
		t.Fatalf("CreateInitialUser returned error: %v", err)
	}
	if _, err := source.CreateAPIKey(ctx, user.ID, "source-key"); err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	manga := mustCreateMangaWithPayload(t, source, ctx, model.MangaPayload{
		PrimaryTitle:    "Round Trip",
		SecondaryTitles: []string{"RT"},
		ArtworkURLs:     []string{"https://example.com/art.jpg"},
		TagGroups:       []model.TagGroup{{Title: "Genres", Tags: []model.Tag{{Title: "Drama"}}}},
	})
	chapter := mustCreateChapterForManga(t, source, ctx, manga.ID, 1, "Round Trip Chapter", "2024-05-01")
	collection, err := source.CreateCollection(ctx, model.CollectionPayload{Title: "Reading"})
	if err != nil {
		t.Fatalf("CreateCollection returned error: %v", err)
	}
	if err := source.ReplaceCollectionManga(ctx, collection.ID, model.CollectionMangaPayload{MangaIDs: []string{manga.ID}}); err != nil {
		t.Fatalf("ReplaceCollectionManga returned error: %v", err)
	}
	if _, err := source.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Manual",
		Type:  "simpleCarousel",
		Mode:  "manual",
		Items: []model.DiscoverSectionItem{{
			Type:     "simpleCarouselItem",
			MangaID:  manga.ID,
			ImageURL: "https://example.com/item.jpg",
			Title:    manga.PrimaryTitle,
			Subtitle: chapter.Title,
		}},
	}); err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	var backup bytes.Buffer
	if err := source.ExportBackupArchive(ctx, &backup); err != nil {
		t.Fatalf("ExportBackupArchive returned error: %v", err)
	}
	target := newTestStore(t)
	if _, err := target.RestoreBackupArchive(ctx, bytes.NewReader(backup.Bytes())); err != nil {
		t.Fatalf("RestoreBackupArchive returned error: %v", err)
	}
	var restored bytes.Buffer
	if err := target.ExportBackupArchive(ctx, &restored); err != nil {
		t.Fatalf("ExportBackupArchive restored returned error: %v", err)
	}

	before := readBackupArchiveFiles(t, backup.Bytes())
	after := readBackupArchiveFiles(t, restored.Bytes())
	delete(before, "manifest.json")
	delete(after, "manifest.json")
	if !mapsEqual(before, after) {
		t.Fatalf("restored backup tree differs")
	}
}

func TestRestoreBackupReplacesLibraryDataAndPreservesAuth(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	user, err := store.CreateInitialUser(ctx, "restore-user", "secret123")
	if err != nil {
		t.Fatalf("CreateInitialUser returned error: %v", err)
	}
	if _, err := store.CreateAPIKey(ctx, user.ID, "restore-key"); err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	manga := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Keep"})
	backup, err := store.ExportBackup(ctx)
	if err != nil {
		t.Fatalf("ExportBackup returned error: %v", err)
	}
	extra := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Remove"})

	result, err := store.RestoreBackup(ctx, backup)
	if err != nil {
		t.Fatalf("RestoreBackup returned error: %v", err)
	}
	if result.MangaCount != 1 {
		t.Fatalf("MangaCount = %d, want 1", result.MangaCount)
	}
	if _, err := store.GetManga(ctx, manga.ID); err != nil {
		t.Fatalf("expected restored manga %s: %v", manga.ID, err)
	}
	if _, err := store.GetManga(ctx, extra.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetManga extra err = %v, want ErrNotFound", err)
	}
	keys, err := store.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys returned error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("api key count = %d, want 1", len(keys))
	}
}

func TestRestoreBackupValidatesPayload(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if _, err := store.RestoreBackup(ctx, &model.Backup{SchemaVersion: model.BackupSchemaVersion + 1}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsupported schema err = %v, want ErrValidation", err)
	}
	if _, err := store.RestoreBackup(ctx, &model.Backup{
		SchemaVersion: model.BackupSchemaVersion,
		Chapters: []model.BackupChapter{{
			ID:          "orphan",
			MangaID:     "missing",
			ChapNum:     1,
			LastUpdated: "2024-01-01T00:00:00Z",
		}},
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing manga err = %v, want ErrValidation", err)
	}
	if _, err := store.RestoreBackup(ctx, &model.Backup{
		SchemaVersion: model.BackupSchemaVersion,
		Manga: []model.BackupManga{{
			ID:           "bad-date",
			PrimaryTitle: "Bad Date",
			CreatedAt:    "nope",
			UpdatedAt:    "2024-01-01T00:00:00Z",
		}},
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad date err = %v, want ErrValidation", err)
	}
}

func marshalBackupJSON(t *testing.T, backup *model.Backup) []byte {
	t.Helper()

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(backup); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	return buf.Bytes()
}

func readBackupArchiveFiles(t *testing.T, body []byte) map[string][]byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("NewReader returned error: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%s) returned error: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s) returned error: %v", file.Name, err)
		}
		files[file.Name] = data
	}
	return files
}

func mapsEqual(left map[string][]byte, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok || !bytes.Equal(leftValue, rightValue) {
			return false
		}
	}
	return true
}
