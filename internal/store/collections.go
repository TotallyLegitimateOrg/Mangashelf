package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
	"github.com/google/uuid"
)

func (s *Store) ListCollections(ctx context.Context) ([]model.Collection, error) {
	rows, err := s.queries.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]model.Collection, 0, len(rows))
	for _, row := range rows {
		items = append(items, collectionFromListRow(row))
	}
	return items, nil
}

func (s *Store) CreateCollection(ctx context.Context, payload model.CollectionPayload) (*model.Collection, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	id := uuid.NewString()
	now := nowUnix()
	if err := s.queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID: id, Title: title, SortOrder: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return nil, err
	}
	return s.getCollection(ctx, id)
}

func (s *Store) UpdateCollection(ctx context.Context, id string, payload model.CollectionPayload) (*model.Collection, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if err := s.queries.UpdateCollection(ctx, gen.UpdateCollectionParams{ID: id, Title: title, UpdatedAt: nowUnix()}); err != nil {
		return nil, err
	}
	return s.getCollection(ctx, id)
}

func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	if _, err := s.getCollection(ctx, id); err != nil {
		return err
	}
	return s.queries.DeleteCollection(ctx, id)
}

func (s *Store) ListCollectionManga(ctx context.Context, id string) ([]model.Manga, error) {
	if _, err := s.getCollection(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListCollectionMangaSummaries(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildMangaList(ctx, collectionRowsToSearchRows(rows))
}

func (s *Store) ReplaceCollectionManga(ctx context.Context, id string, payload model.CollectionMangaPayload) error {
	if _, err := s.getCollection(ctx, id); err != nil {
		return err
	}
	now := nowUnix()
	return s.withTx(ctx, func(q *gen.Queries) error {
		if err := q.DeleteCollectionManga(ctx, id); err != nil {
			return err
		}
		for index, mangaID := range payload.MangaIDs {
			if err := q.InsertCollectionManga(ctx, gen.InsertCollectionMangaParams{
				CollectionID: id, MangaID: mangaID, SortOrder: int64(index), CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ApplyCollectionChanges(ctx context.Context, id string, payload model.CollectionChangesPayload) error {
	if _, err := s.getCollection(ctx, id); err != nil {
		return err
	}
	now := nowUnix()
	return s.withTx(ctx, func(q *gen.Queries) error {
		for _, mangaID := range payload.Deletions {
			if err := q.RemoveMangaFromCollection(ctx, gen.RemoveMangaFromCollectionParams{CollectionID: id, MangaID: mangaID}); err != nil {
				return err
			}
		}
		for _, mangaID := range payload.Additions {
			if err := q.AddMangaToCollection(ctx, gen.AddMangaToCollectionParams{CollectionID: id, MangaID: mangaID, CollectionID_2: id, CreatedAt: now}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ReorderCollections(ctx context.Context, order []string) error {
	now := nowUnix()
	return s.withTx(ctx, func(q *gen.Queries) error {
		for index, id := range order {
			if err := q.SetCollectionSortOrder(ctx, gen.SetCollectionSortOrderParams{ID: id, SortOrder: int64(index), UpdatedAt: now}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) getCollection(ctx context.Context, id string) (*model.Collection, error) {
	row, err := s.queries.GetCollectionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item := collectionFromGetRow(row)
	return &item, nil
}

func collectionFromListRow(row gen.ListCollectionsRow) model.Collection {
	return model.Collection{ID: row.ID, Title: row.Title, SortOrder: int(row.SortOrder), MangaCount: int(row.MangaCount), CreatedAt: isoString(row.CreatedAt), UpdatedAt: isoString(row.UpdatedAt)}
}

func collectionFromGetRow(row gen.GetCollectionByIDRow) model.Collection {
	return model.Collection{ID: row.ID, Title: row.Title, SortOrder: int(row.SortOrder), MangaCount: int(row.MangaCount), CreatedAt: isoString(row.CreatedAt), UpdatedAt: isoString(row.UpdatedAt)}
}

func collectionRowsToSearchRows(rows []gen.ListCollectionMangaSummariesRow) []gen.SearchMangaSummariesRow {
	result := make([]gen.SearchMangaSummariesRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, gen.SearchMangaSummariesRow{
			ID: row.ID, PrimaryTitle: row.PrimaryTitle, Synopsis: row.Synopsis, ThumbnailUrl: row.ThumbnailUrl, BannerUrl: row.BannerUrl, ContentRating: row.ContentRating, Status: row.Status, Artist: row.Artist, Author: row.Author, Rating: row.Rating, ShareUrl: row.ShareUrl, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ChapterCount: row.ChapterCount,
		})
	}
	return result
}
