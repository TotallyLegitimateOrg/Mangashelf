package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func TestCreateDiscoverSectionRequiresAtLeastOneItem(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	_, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title:    "Empty section",
		Subtitle: "",
		Type:     "simpleCarousel",
		Items:    []model.DiscoverSectionItem{},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateDiscoverSection error = %v, want ErrValidation", err)
	}
}

func TestLiveDiscoverSectionGeneratesTitlesAtoZ(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Zeta"})
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Alpha"})
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Beta"})

	section, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "A-Z Titles",
		Type:  "simpleCarousel",
		Mode:  "live",
		LiveRule: &model.DiscoverLiveRule{
			Preset: "title_asc",
			Limit:  2,
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	if section.Mode != "live" {
		t.Fatalf("section mode = %q, want live", section.Mode)
	}
	if got := len(section.Items); got != 2 {
		t.Fatalf("live section item count = %d, want 2", got)
	}
	if section.Items[0].Title != "Alpha" || section.Items[1].Title != "Beta" {
		t.Fatalf("live section titles = %q, %q, want Alpha then Beta", section.Items[0].Title, section.Items[1].Title)
	}
}

func TestLiveDiscoverSectionGeneratesTopRatedProminentItems(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Solid", Rating: floatPtr(7.5)})
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Great", Rating: floatPtr(9.8)})
	mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Good", Rating: floatPtr(8.1)})

	section, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Top Rated",
		Type:  "prominentCarousel",
		Mode:  "live",
		LiveRule: &model.DiscoverLiveRule{
			Preset: "rating_desc",
			Limit:  2,
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	if got := len(section.Items); got != 2 {
		t.Fatalf("live section item count = %d, want 2", got)
	}
	if section.Items[0].Type != "prominentCarouselItem" {
		t.Fatalf("first item type = %q, want prominentCarouselItem", section.Items[0].Type)
	}
	if section.Items[0].Title != "Great" || section.Items[1].Title != "Good" {
		t.Fatalf("live section titles = %q, %q, want Great then Good", section.Items[0].Title, section.Items[1].Title)
	}
}

func TestLiveDiscoverSectionGeneratesLatestChapters(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	first := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "First"})
	second := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Second"})

	older := mustCreateChapterForManga(t, store, ctx, first.ID, 1, "First Chapter", "2024-01-05")
	newer := mustCreateChapterForManga(t, store, ctx, second.ID, 2, "Second Chapter", "2024-02-10")

	section, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Latest Chapters",
		Type:  "chapterUpdates",
		Mode:  "live",
		LiveRule: &model.DiscoverLiveRule{
			Preset: "latest_chapters",
			Limit:  5,
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	if got := len(section.Items); got != 2 {
		t.Fatalf("live section item count = %d, want 2", got)
	}
	if section.Items[0].ChapterID != newer.ID || section.Items[1].ChapterID != older.ID {
		t.Fatalf("live chapter order = %q, %q, want %q then %q", section.Items[0].ChapterID, section.Items[1].ChapterID, newer.ID, older.ID)
	}
	if section.Items[0].Title != "Second" || section.Items[0].Subtitle != "Second Chapter" {
		t.Fatalf("first live chapter item = %+v, want Second / Second Chapter", section.Items[0])
	}
}

func TestLiveDiscoverSectionGeneratesTopGenres(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	mustCreateMangaWithPayload(t, store, ctx, taggedMangaPayload("One", "Action"))
	mustCreateMangaWithPayload(t, store, ctx, taggedMangaPayload("Two", "Action"))
	mustCreateMangaWithPayload(t, store, ctx, taggedMangaPayload("Three", "Comedy"))

	section, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Top Genres",
		Type:  "genres",
		Mode:  "live",
		LiveRule: &model.DiscoverLiveRule{
			Preset: "genres_top",
			Limit:  2,
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	if got := len(section.Items); got != 2 {
		t.Fatalf("live section item count = %d, want 2", got)
	}
	if section.Items[0].Name != "Action" {
		t.Fatalf("first genre item name = %q, want Action", section.Items[0].Name)
	}
	if section.Items[0].SearchQuery == nil || len(section.Items[0].SearchQuery.Filters) != 1 {
		t.Fatalf("first genre item query = %+v, want one search filter", section.Items[0].SearchQuery)
	}
	if got := string(section.Items[0].SearchQuery.Filters[0].Value); got != `"Genres:Action"` {
		t.Fatalf("first genre filter value = %q, want %q", got, `"Genres:Action"`)
	}
}

func TestManualDiscoverSectionRoundTripsStoredItems(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := mustCreateMangaWithPayload(t, store, ctx, model.MangaPayload{PrimaryTitle: "Manual"})

	section, err := store.CreateDiscoverSection(ctx, model.DiscoverSectionPayload{
		Title: "Manual Picks",
		Type:  "simpleCarousel",
		Mode:  "manual",
		Items: []model.DiscoverSectionItem{
			{
				ID:       "manual-item",
				Type:     "simpleCarouselItem",
				MangaID:  manga.ID,
				ImageURL: "https://example.com/manual.jpg",
				Title:    manga.PrimaryTitle,
				Subtitle: "Stored",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDiscoverSection returned error: %v", err)
	}

	if section.Mode != "manual" {
		t.Fatalf("section mode = %q, want manual", section.Mode)
	}
	if got := len(section.Items); got != 1 {
		t.Fatalf("manual section item count = %d, want 1", got)
	}
	if section.Items[0].ID != "manual-item" || section.Items[0].Subtitle != "Stored" {
		t.Fatalf("manual item = %+v, want stored item payload", section.Items[0])
	}
}

func mustCreateMangaWithPayload(t *testing.T, store *Store, ctx context.Context, payload model.MangaPayload) *model.Manga {
	t.Helper()

	manga, err := store.CreateManga(ctx, payload)
	if err != nil {
		t.Fatalf("CreateManga(%q) returned error: %v", payload.PrimaryTitle, err)
	}
	return manga
}

func mustCreateChapterForManga(t *testing.T, store *Store, ctx context.Context, mangaID string, chapNum float64, title string, publishDate string) *model.ChapterDetail {
	t.Helper()

	chapter, err := store.CreateChapter(ctx, mangaID, model.ChapterPayload{
		ChapNum:     chapNum,
		Title:       title,
		PublishDate: &publishDate,
		Pages:       []string{fmt.Sprintf("https://example.com/%s-%v.jpg", mangaID, chapNum)},
	})
	if err != nil {
		t.Fatalf("CreateChapter(%q, %v) returned error: %v", mangaID, chapNum, err)
	}
	return chapter
}

func taggedMangaPayload(title string, tag string) model.MangaPayload {
	return model.MangaPayload{
		PrimaryTitle: title,
		TagGroups: []model.TagGroup{
			{
				Title: "Genres",
				Tags:  []model.Tag{{Title: tag}},
			},
		},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
