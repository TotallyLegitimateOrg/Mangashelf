package store

import (
	"context"
	"errors"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func TestSearchMangaAdvancedFiltersAndSorting(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	alpha := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle:    "Alpha",
		SecondaryTitles: []string{"First Alias"},
		ContentRating:   "SAFE",
		Status:          "Ongoing",
		Rating:          floatPointer(8.5),
		TagGroups:       []model.TagGroup{{Title: "Genres", Tags: []model.Tag{{Title: "Action"}}}},
		AdditionalInfo:  []model.InfoEntry{},
		ArtworkURLs:     []string{},
		ThumbnailURL:    "",
		BannerURL:       "",
		Synopsis:        "",
		Author:          "",
		Artist:          "",
		ShareURL:        "",
	})
	beta := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle:    "Beta",
		SecondaryTitles: []string{"Second Alias"},
		ContentRating:   "MATURE",
		Status:          "Completed",
		Rating:          floatPointer(6.25),
		TagGroups:       []model.TagGroup{{Title: "Genres", Tags: []model.Tag{{Title: "Drama"}}}},
	})

	cases := []struct {
		name string
		opts model.MangaSearchOptions
		want string
	}{
		{name: "title", opts: model.MangaSearchOptions{Query: "alp"}, want: alpha.ID},
		{name: "secondary title", opts: model.MangaSearchOptions{Query: "second"}, want: beta.ID},
		{name: "content rating", opts: model.MangaSearchOptions{ContentRating: []string{"MATURE"}}, want: beta.ID},
		{name: "status", opts: model.MangaSearchOptions{Status: []string{"Ongoing"}}, want: alpha.ID},
		{name: "tag", opts: model.MangaSearchOptions{Tags: []string{"Genres:Action"}}, want: alpha.ID},
		{name: "rating range", opts: model.MangaSearchOptions{MinRating: floatPointer(8), MaxRating: floatPointer(9)}, want: alpha.ID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := store.SearchManga(ctx, tc.opts)
			if err != nil {
				t.Fatalf("SearchManga returned error: %v", err)
			}
			if len(items) != 1 || items[0].ID != tc.want {
				t.Fatalf("SearchManga returned %+v, want only %s", ids(items), tc.want)
			}
		})
	}

	for _, sortOption := range []string{"updated_desc", "updated_asc", "title_asc", "title_desc", "rating_desc", "rating_asc", "chapters_desc", "chapters_asc"} {
		t.Run("sort "+sortOption, func(t *testing.T) {
			items, err := store.SearchManga(ctx, model.MangaSearchOptions{Sort: sortOption})
			if err != nil {
				t.Fatalf("SearchManga sort %s returned error: %v", sortOption, err)
			}
			if len(items) != 2 {
				t.Fatalf("SearchManga sort %s returned %d items, want 2", sortOption, len(items))
			}
		})
	}
}

func TestSearchMangaSortUsesSQLTieBreaks(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	newerAlpha := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle: "Alpha",
		Rating:       floatPointer(9.0),
	})
	olderAlpha := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle: "Alpha",
		Rating:       floatPointer(9.0),
	})
	nilRated := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle: "Gamma",
	})
	chapterTwin := createTestManga(t, store, ctx, model.MangaPayload{
		PrimaryTitle: "Zulu",
		Rating:       floatPointer(7.0),
	})

	mustCreateChapterForManga(t, store, ctx, newerAlpha.ID, 1, "Alpha 1", "2024-01-01")
	mustCreateChapterForManga(t, store, ctx, olderAlpha.ID, 1, "Alpha 2", "2024-01-02")
	mustCreateChapterForManga(t, store, ctx, chapterTwin.ID, 1, "Zulu 1", "2024-01-03")

	setUpdatedAt(t, store, ctx, newerAlpha.ID, 400)
	setUpdatedAt(t, store, ctx, olderAlpha.ID, 300)
	setUpdatedAt(t, store, ctx, nilRated.ID, 200)
	setUpdatedAt(t, store, ctx, chapterTwin.ID, 100)

	t.Run("title asc falls back to updated desc", func(t *testing.T) {
		items, err := store.SearchManga(ctx, model.MangaSearchOptions{Sort: "title_asc"})
		if err != nil {
			t.Fatalf("SearchManga title_asc returned error: %v", err)
		}
		if got := ids(items[:2]); got[0] != newerAlpha.ID || got[1] != olderAlpha.ID {
			t.Fatalf("title_asc first two ids = %v, want [%s %s]", got, newerAlpha.ID, olderAlpha.ID)
		}
	})

	t.Run("rating desc falls back to updated desc", func(t *testing.T) {
		items, err := store.SearchManga(ctx, model.MangaSearchOptions{Sort: "rating_desc"})
		if err != nil {
			t.Fatalf("SearchManga rating_desc returned error: %v", err)
		}
		if got := ids(items[:2]); got[0] != newerAlpha.ID || got[1] != olderAlpha.ID {
			t.Fatalf("rating_desc first two ids = %v, want [%s %s]", got, newerAlpha.ID, olderAlpha.ID)
		}
	})

	t.Run("rating asc keeps nil ratings first", func(t *testing.T) {
		items, err := store.SearchManga(ctx, model.MangaSearchOptions{Sort: "rating_asc"})
		if err != nil {
			t.Fatalf("SearchManga rating_asc returned error: %v", err)
		}
		if items[0].ID != nilRated.ID {
			t.Fatalf("rating_asc first id = %s, want %s", items[0].ID, nilRated.ID)
		}
	})

	t.Run("chapters desc falls back to updated desc", func(t *testing.T) {
		items, err := store.SearchManga(ctx, model.MangaSearchOptions{Sort: "chapters_desc"})
		if err != nil {
			t.Fatalf("SearchManga chapters_desc returned error: %v", err)
		}
		if got := ids(items[:3]); got[0] != newerAlpha.ID || got[1] != olderAlpha.ID || got[2] != chapterTwin.ID {
			t.Fatalf("chapters_desc first three ids = %v, want [%s %s %s]", got, newerAlpha.ID, olderAlpha.ID, chapterTwin.ID)
		}
	})
}

func TestCollectionsCRUDMembershipAndCascades(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	first := newTestManga(t, store, ctx)
	second := createTestManga(t, store, ctx, model.MangaPayload{PrimaryTitle: "Second Manga"})

	collection, err := store.CreateCollection(ctx, model.CollectionPayload{Title: "Reading"})
	if err != nil {
		t.Fatalf("CreateCollection returned error: %v", err)
	}
	if collection.Title != "Reading" {
		t.Fatalf("collection title = %q, want Reading", collection.Title)
	}

	updated, err := store.UpdateCollection(ctx, collection.ID, model.CollectionPayload{Title: "Favorites"})
	if err != nil {
		t.Fatalf("UpdateCollection returned error: %v", err)
	}
	if updated.Title != "Favorites" {
		t.Fatalf("updated title = %q, want Favorites", updated.Title)
	}

	if err := store.ReplaceCollectionManga(ctx, collection.ID, model.CollectionMangaPayload{MangaIDs: []string{first.ID, second.ID}}); err != nil {
		t.Fatalf("ReplaceCollectionManga returned error: %v", err)
	}
	manga, err := store.ListCollectionManga(ctx, collection.ID)
	if err != nil {
		t.Fatalf("ListCollectionManga returned error: %v", err)
	}
	if len(manga) != 2 {
		t.Fatalf("ListCollectionManga returned %d items, want 2", len(manga))
	}

	if err := store.ApplyCollectionChanges(ctx, collection.ID, model.CollectionChangesPayload{Deletions: []string{first.ID}}); err != nil {
		t.Fatalf("ApplyCollectionChanges delete returned error: %v", err)
	}
	manga, err = store.ListCollectionManga(ctx, collection.ID)
	if err != nil {
		t.Fatalf("ListCollectionManga after delete returned error: %v", err)
	}
	if len(manga) != 1 || manga[0].ID != second.ID {
		t.Fatalf("collection manga after delete = %+v, want second only", ids(manga))
	}

	if err := store.ApplyCollectionChanges(ctx, collection.ID, model.CollectionChangesPayload{Additions: []string{first.ID}}); err != nil {
		t.Fatalf("ApplyCollectionChanges add returned error: %v", err)
	}
	if err := store.DeleteManga(ctx, first.ID); err != nil {
		t.Fatalf("DeleteManga returned error: %v", err)
	}
	manga, err = store.ListCollectionManga(ctx, collection.ID)
	if err != nil {
		t.Fatalf("ListCollectionManga after manga cascade returned error: %v", err)
	}
	if len(manga) != 1 || manga[0].ID != second.ID {
		t.Fatalf("collection manga after manga cascade = %+v, want second only", ids(manga))
	}

	if err := store.DeleteCollection(ctx, collection.ID); err != nil {
		t.Fatalf("DeleteCollection returned error: %v", err)
	}
	if _, err := store.ListCollectionManga(ctx, collection.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListCollectionManga after collection delete error = %v, want ErrNotFound", err)
	}
}

func createTestManga(t *testing.T, store *Store, ctx context.Context, payload model.MangaPayload) *model.Manga {
	t.Helper()
	manga, err := store.CreateManga(ctx, payload)
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	return manga
}

func floatPointer(value float64) *float64 {
	return &value
}

func ids(items []model.Manga) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ID)
	}
	return result
}

func setUpdatedAt(t *testing.T, store *Store, ctx context.Context, mangaID string, updatedAt int64) {
	t.Helper()

	if err := store.queries.TouchMangaUpdatedAt(ctx, gen.TouchMangaUpdatedAtParams{
		UpdatedAt: updatedAt,
		ID:        mangaID,
	}); err != nil {
		t.Fatalf("TouchMangaUpdatedAt(%q, %d) returned error: %v", mangaID, updatedAt, err)
	}
}
