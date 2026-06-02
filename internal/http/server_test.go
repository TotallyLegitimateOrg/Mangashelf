package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestBackupEndpointReturnsZipAttachment(t *testing.T) {
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

	firstFiles := readHTTPBackupArchiveFiles(t, firstBody)
	secondFiles := readHTTPBackupArchiveFiles(t, secondBody)
	for _, name := range []string{"manifest.json", "manga/" + manga.ID + ".json"} {
		if _, ok := firstFiles[name]; !ok {
			t.Fatalf("backup response missing entry %s", name)
		}
	}
	delete(firstFiles, "manifest.json")
	delete(secondFiles, "manifest.json")
	if !byteMapsEqual(firstFiles, secondFiles) {
		t.Fatalf("backup entity files differ across repeated requests")
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
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/zip")
	}
	wantFilename := "mangashelf-backup-" + time.Now().UTC().Format(time.DateOnly) + ".zip"
	wantDisposition := fmt.Sprintf(`attachment; filename="%s"`, wantFilename)
	if got := rec.Header().Get("Content-Disposition"); got != wantDisposition {
		t.Fatalf("Content-Disposition = %q, want %q", got, wantDisposition)
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

	before := readHTTPBackupArchiveFiles(t, body)
	after := readHTTPBackupArchiveFiles(t, requestBackup(t, server, apiKey))
	delete(before, "manifest.json")
	delete(after, "manifest.json")
	if !byteMapsEqual(before, after) {
		t.Fatalf("backup after restore differs")
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

func readHTTPBackupArchiveFiles(t *testing.T, body []byte) map[string][]byte {
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

func byteMapsEqual(left map[string][]byte, right map[string][]byte) bool {
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
