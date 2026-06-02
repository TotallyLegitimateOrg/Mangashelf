package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/auth"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
	dbpkg "github.com/TotallyLegitimateOrg/Mangashelf/internal/db"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/store"
)

func TestBackupEndpointRequiresAuth(t *testing.T) {
	_, server, _ := newTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/backups/export", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBackupEndpointReturnsJSONAttachment(t *testing.T) {
	dataStore, server, apiKey := newTestHTTPServer(t)
	ctx := context.Background()

	manga, err := dataStore.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Exported"})
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	if _, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{
		ChapNum: 1,
		Title:   "Chapter 1",
		Pages:   []string{"https://example.com/chapter-1.jpg"},
	}); err != nil {
		t.Fatalf("CreateChapter returned error: %v", err)
	}

	firstBody := requestBackup(t, server, apiKey)
	secondBody := requestBackup(t, server, apiKey)

	if !bytes.Equal(firstBody, secondBody) {
		t.Fatalf("backup responses differ across repeated requests\nfirst:  %s\nsecond: %s", firstBody, secondBody)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(firstBody, &raw); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	for _, key := range []string{"schemaVersion", "manga", "chapters", "collections", "discoverSections", "chapterSources"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("backup response missing top-level key %q", key)
		}
	}
	if _, ok := raw["backup"]; ok {
		t.Fatalf("backup response should be raw JSON, not wrapped in a backup envelope")
	}

	var decoded model.Backup
	if err := json.Unmarshal(firstBody, &decoded); err != nil {
		t.Fatalf("Unmarshal backup returned error: %v", err)
	}
	if decoded.SchemaVersion != model.BackupSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", decoded.SchemaVersion, model.BackupSchemaVersion)
	}
	if len(decoded.Manga) != 1 || decoded.Manga[0].ID != manga.ID {
		t.Fatalf("decoded manga = %+v, want one exported manga %q", decoded.Manga, manga.ID)
	}
}

func TestBulkChapterMetadataEndpointUpdatesChapters(t *testing.T) {
	dataStore, server, apiKey := newTestHTTPServer(t)
	ctx := context.Background()
	manga, err := dataStore.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Bulk"})
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	first, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{ChapNum: 1, Version: "Default"})
	if err != nil {
		t.Fatalf("CreateChapter first returned error: %v", err)
	}
	second, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{ChapNum: 2, Version: "Default"})
	if err != nil {
		t.Fatalf("CreateChapter second returned error: %v", err)
	}

	body, err := json.Marshal(model.ChapterBulkMetadataPayload{
		ChapterIDs: []string{first.ID, second.ID},
		LangCode:   testStringPointer("FR"),
		Version:    testStringPointer("Scanlation"),
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/manga/"+manga.ID+"/chapters/bulk-metadata", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var result model.ChapterBulkMetadataResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if result.UpdatedCount != 2 || len(result.Chapters) != 2 {
		t.Fatalf("result = %+v, want two updated chapters", result)
	}
	if result.Chapters[0].LangCode != "FR" || result.Chapters[0].Version != "Scanlation" {
		t.Fatalf("first chapter = %+v, want updated language and version", result.Chapters[0])
	}
}

func TestBulkChapterMetadataEndpointRejectsConflicts(t *testing.T) {
	dataStore, server, apiKey := newTestHTTPServer(t)
	ctx := context.Background()
	manga, err := dataStore.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Bulk Conflict"})
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	first, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{ChapNum: 1, LangCode: "EN", Version: "Default"})
	if err != nil {
		t.Fatalf("CreateChapter first returned error: %v", err)
	}
	if _, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{ChapNum: 1, LangCode: "FR", Version: "Default"}); err != nil {
		t.Fatalf("CreateChapter second returned error: %v", err)
	}

	body, err := json.Marshal(model.ChapterBulkMetadataPayload{
		ChapterIDs: []string{first.ID},
		LangCode:   testStringPointer("FR"),
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/manga/"+manga.ID+"/chapters/bulk-metadata", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "duplicate chapter identities") {
		t.Fatalf("body = %s, want duplicate identity message", rec.Body.String())
	}
}

func newTestHTTPServer(t *testing.T) (*store.Store, *Server, string) {
	t.Helper()

	cfg := config.Config{
		DatabasePath:    filepath.Join(t.TempDir(), "mangashelf-http-test.db"),
		JWTSecret:       "test-secret",
		DevWebURL:       "http://example.com",
		DevExtensionURL: "http://example.com/extension",
	}
	database, err := dbpkg.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.SQL.Close()
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dataStore := store.New(database.SQL, logger)
	authManager := auth.New(cfg.JWTSecret, dataStore)
	server, err := New(cfg, logger, dataStore, authManager)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	user, err := dataStore.CreateInitialUser(context.Background(), "http-user", "secret123")
	if err != nil {
		t.Fatalf("CreateInitialUser returned error: %v", err)
	}
	apiKey, err := dataStore.CreateAPIKey(context.Background(), user.ID, "http-key")
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	return dataStore, server, apiKey
}

func testStringPointer(value string) *string {
	return &value
}

func requestBackup(t *testing.T, server *Server, apiKey string) []byte {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/backups/export", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="mangashelf-backup.json"` {
		t.Fatalf("Content-Disposition = %q, want %q", got, `attachment; filename="mangashelf-backup.json"`)
	}

	return rec.Body.Bytes()
}

func TestBackupRestoreEndpointRequiresAuth(t *testing.T) {
	_, server, _ := newTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/backups/restore", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBackupRestoreEndpointReplacesLibraryData(t *testing.T) {
	dataStore, server, apiKey := newTestHTTPServer(t)
	ctx := context.Background()

	manga, err := dataStore.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Restored"})
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	if _, err := dataStore.CreateChapter(ctx, manga.ID, model.ChapterPayload{
		ChapNum: 1,
		Title:   "Restored Chapter",
		Pages:   []string{"https://example.com/restored.jpg"},
	}); err != nil {
		t.Fatalf("CreateChapter returned error: %v", err)
	}
	body := requestBackup(t, server, apiKey)

	if _, err := dataStore.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Existing"}); err != nil {
		t.Fatalf("CreateManga existing returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backups/restore", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var result model.BackupRestoreResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal restore result returned error: %v", err)
	}
	if result.MangaCount != 1 || result.ChapterCount != 1 {
		t.Fatalf("restore result = %+v, want one manga and chapter", result)
	}

	after := requestBackup(t, server, apiKey)
	if !bytes.Equal(body, after) {
		t.Fatalf("backup after restore differs\nbefore: %s\nafter:  %s", body, after)
	}
}

func TestBackupRestoreEndpointRejectsInvalidPayload(t *testing.T) {
	_, server, apiKey := newTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/backups/restore", strings.NewReader(`{"schemaVersion":999}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
