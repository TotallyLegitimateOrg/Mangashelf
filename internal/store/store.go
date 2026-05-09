package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrValidation         = errors.New("validation failed")
	ErrSetupAlreadyExists = errors.New("setup already completed")
)

type Store struct {
	db      *sql.DB
	queries *gen.Queries
	log     *slog.Logger
}

type UserIdentity struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func New(db *sql.DB, logger *slog.Logger) *Store {
	return &Store{
		db:      db,
		queries: gen.New(db),
		log:     logger,
	}
}

func (s *Store) withTx(ctx context.Context, fn func(*gen.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := s.queries.WithTx(tx)
	if err := fn(q); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func nowUnix() int64 {
	return time.Now().UTC().Unix()
}

func isoString(unix int64) string {
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func isoStringPtrFromNull(value sql.NullInt64) *string {
	if !value.Valid {
		return nil
	}
	formatted := isoString(value.Int64)
	return &formatted
}

func intPtrFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func floatPtrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func nullFloat(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func nullIntFromTime(value *time.Time) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value.UTC().Unix(), Valid: true}
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func parseTimePointer(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		utc := parsed.UTC()
		return &utc, nil
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		utc := parsed.UTC()
		return &utc, nil
	}
	return nil, fmt.Errorf("%w: invalid date %q", ErrValidation, trimmed)
}

func localChapterOrigin() model.ChapterOrigin {
	return model.ChapterOrigin{
		Kind:     "local",
		Mode:     "local",
		ReadOnly: false,
	}
}

func importedChapterOrigin(provider string, mode string, sourceID *string, sourceStatus *string, chapterKey *string) model.ChapterOrigin {
	return model.ChapterOrigin{
		Kind:         provider,
		Mode:         mode,
		ReadOnly:     false,
		Provider:     stringPointerOrNil(provider),
		SourceID:     sourceID,
		SourceStatus: sourceStatus,
		ChapterKey:   chapterKey,
	}
}

func proxyChapterOrigin(source model.ChapterSource, chapterKey string) model.ChapterOrigin {
	sourceID := source.ID
	provider := source.Provider
	status := source.Status
	return model.ChapterOrigin{
		Kind:         source.Provider,
		Mode:         "proxy",
		ReadOnly:     true,
		Provider:     &provider,
		SourceID:     &sourceID,
		SourceStatus: &status,
		ChapterKey:   stringPointerOrNil(chapterKey),
	}
}

func stringPointerOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func sortChapters(chapters []model.ChapterListItem) {
	sort.SliceStable(chapters, func(i, j int) bool {
		a, b := chapters[i], chapters[j]
		if a.SortingIndex == nil && b.SortingIndex != nil {
			return false
		}
		if a.SortingIndex != nil && b.SortingIndex == nil {
			return true
		}
		if a.SortingIndex != nil && b.SortingIndex != nil && *a.SortingIndex != *b.SortingIndex {
			return *a.SortingIndex < *b.SortingIndex
		}
		if a.ChapNum != b.ChapNum {
			return a.ChapNum < b.ChapNum
		}
		return a.LastUpdated < b.LastUpdated
	})
}

func decodeJSON[T any](raw sql.NullString, fallback T) T {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return fallback
	}
	var value T
	if err := json.Unmarshal([]byte(raw.String), &value); err != nil {
		return fallback
	}
	return value
}

func isNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
