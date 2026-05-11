package store

import (
	"bytes"
	"context"
	"encoding/json"
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
				ID:       "manual-item",
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
	if got := sectionsByID[manualSection.ID]; len(got.Items) != 1 || got.Items[0].ID != "manual-item" {
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
