package store

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
	dbpkg "github.com/TotallyLegitimateOrg/Mangashelf/internal/db"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/importer"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
)

func TestProxySourceListsAndFetchesLiveChapters(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
		"2": {"https://example.com/ch2-p1.jpg", "https://example.com/ch2-p2.jpg"},
	}))

	result, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "proxy", fixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport returned error: %v", err)
	}
	source := result.Source
	if source == nil {
		t.Fatalf("CreateChapterImport returned nil source for proxy mode")
	}
	if source.Mode != "proxy" {
		t.Fatalf("source mode = %q, want proxy", source.Mode)
	}
	if source.Provider != "cubari" {
		t.Fatalf("source provider = %q, want cubari", source.Provider)
	}

	chapters, err := store.ListChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapters returned error: %v", err)
	}
	if got := len(chapters); got != 2 {
		t.Fatalf("ListChapters returned %d chapters, want 2", got)
	}
	if chapters[0].Origin.Mode != "proxy" || chapters[1].Origin.Mode != "proxy" {
		t.Fatalf("ListChapters should return proxy-backed chapters for proxy sources")
	}
	if chapters[0].Origin.Provider == nil || *chapters[0].Origin.Provider != "cubari" {
		t.Fatalf("proxy chapter origin provider = %v, want cubari", chapters[0].Origin.Provider)
	}

	detail, err := store.GetChapter(ctx, manga.ID, chapters[0].ID)
	if err != nil {
		t.Fatalf("GetChapter returned error: %v", err)
	}
	if detail.Origin.Mode != "proxy" {
		t.Fatalf("GetChapter origin mode = %q, want proxy", detail.Origin.Mode)
	}
	if len(detail.Pages) == 0 {
		t.Fatalf("GetChapter returned no pages")
	}
}

func TestSyncSourceUsesImportedChaptersOnly(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
	}))

	result, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "sync", fixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport returned error: %v", err)
	}
	source := result.Source
	if source == nil {
		t.Fatalf("CreateChapterImport returned nil source for sync mode")
	}
	if source.Mode != "sync" {
		t.Fatalf("source mode = %q, want sync", source.Mode)
	}
	logs, err := store.ListChapterSourceSyncLogs(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSourceSyncLogs returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].Status != "success" || logs[0].InsertedCount != 1 {
		t.Fatalf("initial sync logs = %+v, want one success log with 1 inserted", logs)
	}

	fixture.SetBody(cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1-updated.jpg"},
		"2": {"https://example.com/ch2-p1.jpg"},
	}))

	chapters, err := store.ListChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapters returned error: %v", err)
	}
	if got := len(chapters); got != 1 {
		t.Fatalf("ListChapters returned %d chapters, want 1", got)
	}
	if chapters[0].Origin.Mode != "sync" {
		t.Fatalf("ListChapters origin mode = %q, want sync", chapters[0].Origin.Mode)
	}
	if chapters[0].Origin.Provider == nil || *chapters[0].Origin.Provider != "cubari" {
		t.Fatalf("stored sync chapter origin provider = %v, want cubari", chapters[0].Origin.Provider)
	}

	proxyID := importer.CreateProxyChapterID(source.Provider, source.ID, importer.CreateChapterIdentityKey(2, "EN", "Default"))
	if _, err := store.GetChapter(ctx, manga.ID, proxyID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetChapter for sync-source proxy id error = %v, want ErrNotFound", err)
	}

	updatedSource, stats, err := store.SyncChapterSource(ctx, manga.ID, source.ID)
	if err != nil {
		t.Fatalf("SyncChapterSource returned error: %v", err)
	}
	if updatedSource.Status != "ready" {
		t.Fatalf("updated source status = %q, want ready", updatedSource.Status)
	}
	if stats.Inserted != 1 || stats.Updated != 1 || stats.Unchanged != 0 || stats.Skipped != 0 {
		t.Fatalf("SyncChapterSource stats = %+v, want 1 inserted, 1 updated, 0 unchanged, 0 skipped", stats)
	}
	logs, err = store.ListChapterSourceSyncLogs(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSourceSyncLogs after sync returned error: %v", err)
	}
	if len(logs) != 2 || !hasSyncLog(logs, 1, 1, 0, 0) {
		t.Fatalf("sync logs after sync = %+v, want a log with 1 inserted and 1 updated", logs)
	}

	chapters, err = store.ListChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapters after sync returned error: %v", err)
	}
	if got := len(chapters); got != 2 {
		t.Fatalf("ListChapters after sync returned %d chapters, want 2", got)
	}

	detail, err := store.GetChapter(ctx, manga.ID, chapters[0].ID)
	if err != nil {
		t.Fatalf("GetChapter returned error: %v", err)
	}
	if got := detail.Pages[0]; got != "https://example.com/ch1-p1-updated.jpg" {
		t.Fatalf("GetChapter first page = %q, want updated sync page", got)
	}
	if !hasInfoEntry(detail.AdditionalInfo, "Provider", "cubari") {
		t.Fatalf("GetChapter additional info = %+v, want Provider", detail.AdditionalInfo)
	}
	lastUpdated := detail.LastUpdated

	_, stats, err = store.SyncChapterSource(ctx, manga.ID, source.ID)
	if err != nil {
		t.Fatalf("SyncChapterSource no-op returned error: %v", err)
	}
	if stats.Inserted != 0 || stats.Updated != 0 || stats.Unchanged != 2 || stats.Skipped != 0 {
		t.Fatalf("SyncChapterSource no-op stats = %+v, want 0 inserted, 0 updated, 2 unchanged, 0 skipped", stats)
	}
	logs, err = store.ListChapterSourceSyncLogs(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSourceSyncLogs after no-op sync returned error: %v", err)
	}
	if len(logs) != 3 || !hasSyncLog(logs, 0, 0, 2, 0) {
		t.Fatalf("sync logs after no-op sync = %+v, want a log with 2 unchanged", logs)
	}
	detail, err = store.GetChapter(ctx, manga.ID, chapters[0].ID)
	if err != nil {
		t.Fatalf("GetChapter after no-op sync returned error: %v", err)
	}
	if detail.LastUpdated != lastUpdated {
		t.Fatalf("unchanged sync rewrote chapter: lastUpdated = %q, want %q", detail.LastUpdated, lastUpdated)
	}

	if _, err := store.db.ExecContext(ctx, `
		UPDATE chapter_info_entries
		SET info_value = 'stale provider'
		WHERE chapter_id = ? AND info_key = 'Provider'
	`, detail.ID); err != nil {
		t.Fatalf("corrupting chapter info returned error: %v", err)
	}
	_, stats, err = store.SyncChapterSource(ctx, manga.ID, source.ID)
	if err != nil {
		t.Fatalf("SyncChapterSource after info-only change returned error: %v", err)
	}
	if stats.Inserted != 0 || stats.Updated != 1 || stats.Unchanged != 1 || stats.Skipped != 0 {
		t.Fatalf("SyncChapterSource info-only stats = %+v, want 0 inserted, 1 updated, 1 unchanged, 0 skipped", stats)
	}
	detail, err = store.GetChapter(ctx, manga.ID, detail.ID)
	if err != nil {
		t.Fatalf("GetChapter after info-only sync returned error: %v", err)
	}
	if !hasInfoEntry(detail.AdditionalInfo, "Provider", "cubari") {
		t.Fatalf("GetChapter additional info after resync = %+v, want restored Provider", detail.AdditionalInfo)
	}
}

func TestClearChapterSourceSyncLogsRemovesOnlyMatchingSourceLogs(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)

	fixtureOne := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/source-one-ch1.jpg"},
	}))
	resultOne, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "sync", fixtureOne.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport for source one returned error: %v", err)
	}
	if resultOne.Source == nil {
		t.Fatalf("CreateChapterImport for source one returned nil source")
	}

	fixtureTwo := newCubariFixture(t, cubariPayload(map[string][]string{
		"2": {"https://example.com/source-two-ch2.jpg"},
	}))
	resultTwo, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "sync", fixtureTwo.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport for source two returned error: %v", err)
	}
	if resultTwo.Source == nil {
		t.Fatalf("CreateChapterImport for source two returned nil source")
	}

	if err := store.ClearChapterSourceSyncLogs(ctx, manga.ID, resultOne.Source.ID); err != nil {
		t.Fatalf("ClearChapterSourceSyncLogs returned error: %v", err)
	}

	logs, err := store.ListChapterSourceSyncLogs(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSourceSyncLogs returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("remaining sync logs = %+v, want exactly one log", logs)
	}
	if logs[0].SourceID != resultTwo.Source.ID {
		t.Fatalf("remaining sync log source = %q, want %q", logs[0].SourceID, resultTwo.Source.ID)
	}
}

func TestImportOnceCreatesLocalChaptersWithoutSavedSource(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
	}))

	result, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "import_once", fixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport returned error: %v", err)
	}
	if result.Source != nil {
		t.Fatalf("import_once should not return a persisted source")
	}
	if result.InsertedCount != 1 || result.SkippedCount != 0 {
		t.Fatalf("import_once counts = (%d inserted, %d skipped), want (1, 0)", result.InsertedCount, result.SkippedCount)
	}

	sources, err := store.ListChapterSources(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSources returned error: %v", err)
	}
	if got := len(sources); got != 0 {
		t.Fatalf("ListChapterSources returned %d sources, want 0", got)
	}

	chapters, err := store.ListChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapters returned error: %v", err)
	}
	if got := len(chapters); got != 1 {
		t.Fatalf("ListChapters returned %d chapters, want 1", got)
	}
	if chapters[0].Origin.Mode != "import_once" {
		t.Fatalf("ListChapters origin mode = %q, want import_once", chapters[0].Origin.Mode)
	}
	if chapters[0].Origin.Provider == nil || *chapters[0].Origin.Provider != "cubari" {
		t.Fatalf("ListChapters origin provider = %v, want cubari", chapters[0].Origin.Provider)
	}
}

func TestSyncChapterSourceRejectsProxySource(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
	}))

	result, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "proxy", fixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport returned error: %v", err)
	}
	source := result.Source
	if source == nil {
		t.Fatalf("CreateChapterImport returned nil source")
	}

	before, err := store.getChapterSource(ctx, source.ID)
	if err != nil {
		t.Fatalf("getChapterSource returned error: %v", err)
	}

	if _, _, err := store.SyncChapterSource(ctx, manga.ID, source.ID); !errors.Is(err, ErrValidation) {
		t.Fatalf("SyncChapterSource error = %v, want ErrValidation", err)
	}

	after, err := store.getChapterSource(ctx, source.ID)
	if err != nil {
		t.Fatalf("getChapterSource returned error: %v", err)
	}
	if before.LastSyncedAt != nil || after.LastSyncedAt != nil {
		t.Fatalf("proxy source should not gain lastSyncedAt")
	}

	chapters, err := store.listStoredChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("listStoredChapters returned error: %v", err)
	}
	if got := len(chapters); got != 0 {
		t.Fatalf("stored chapter count = %d, want 0", got)
	}
}

func TestRunSyncDueSourcesPersistsSyncLog(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
	}))

	result, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "sync", fixture.URL()))
	if err != nil {
		t.Fatalf("CreateChapterImport returned error: %v", err)
	}
	source := result.Source
	if source == nil {
		t.Fatalf("CreateChapterImport returned nil source for sync mode")
	}

	fixture.SetBody(cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
		"2": {"https://example.com/ch2-p1.jpg"},
	}))

	now := nowUnix()
	if err := store.queries.UpdateChapterSourceSynced(ctx, gen.UpdateChapterSourceSyncedParams{
		LastSyncedAt:         sql.NullInt64{Int64: now - 7200, Valid: true},
		LastSeenChapterCount: sql.NullInt64{Int64: 1, Valid: true},
		Status:               "ready",
		LastError:            "",
		UpdatedAt:            now,
		ID:                   source.ID,
	}); err != nil {
		t.Fatalf("UpdateChapterSourceSynced returned error: %v", err)
	}

	store.RunSyncDueSources(ctx)

	logs, err := store.ListChapterSourceSyncLogs(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSourceSyncLogs returned error: %v", err)
	}
	if len(logs) != 2 || !hasSyncLog(logs, 1, 0, 1, 0) {
		t.Fatalf("scheduled sync logs = %+v, want a log with 1 inserted and 1 unchanged", logs)
	}
}

func TestAddSyncSourceRollsBackOnImportFailure(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)
	fixture := newCubariFixture(t, cubariPayload(map[string][]string{
		"1": {"https://example.com/ch1-p1.jpg"},
	}))

	if _, err := store.db.ExecContext(ctx, `
		CREATE TRIGGER abort_chapter_insert
		BEFORE INSERT ON chapters
		BEGIN
			SELECT RAISE(ABORT, 'chapter insert aborted');
		END;
	`); err != nil {
		t.Fatalf("creating abort trigger: %v", err)
	}

	if _, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("cubari", "sync", fixture.URL())); err == nil {
		t.Fatalf("CreateChapterImport succeeded, want error")
	}

	sources, err := store.queries.ListChapterSources(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListChapterSources returned error: %v", err)
	}
	if got := len(sources); got != 0 {
		t.Fatalf("chapter source count = %d, want 0", got)
	}

	chapters, err := store.queries.ListStoredChapters(ctx, manga.ID)
	if err != nil {
		t.Fatalf("ListStoredChapters returned error: %v", err)
	}
	if got := len(chapters); got != 0 {
		t.Fatalf("stored chapter count = %d, want 0", got)
	}
}

func TestCreateChapterImportRejectsUnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)

	if _, err := store.CreateChapterImport(ctx, manga.ID, chapterImportPayload("other", "proxy", "https://example.com/source.json")); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateChapterImport error = %v, want ErrValidation", err)
	}
}

func TestCreateChapterFromArchiveReportsProgress(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	manga := newTestManga(t, store, ctx)

	previousUploader := uploadArchivePage
	uploadArchivePage = func(_ context.Context, filename string, _ []byte) (string, error) {
		return "https://example.com/" + filename, nil
	}
	t.Cleanup(func() {
		uploadArchivePage = previousUploader
	})

	header := newArchiveUploadHeader(t, map[string]string{
		"002-page.png": "second",
		"001-page.png": "first",
		"notes.txt":    "ignored",
	})

	progressEvents := make([]ArchiveUploadProgress, 0)
	chapter, err := store.CreateChapterFromArchiveWithProgress(ctx, manga.ID, header, model.ChapterPayload{
		ChapNum: 1,
	}, func(progress ArchiveUploadProgress) {
		progressEvents = append(progressEvents, progress)
	})
	if err != nil {
		t.Fatalf("CreateChapterFromArchiveWithProgress returned error: %v", err)
	}
	if chapter == nil {
		t.Fatalf("CreateChapterFromArchiveWithProgress returned nil chapter")
	}
	if got := len(chapter.Pages); got != 2 {
		t.Fatalf("chapter pages = %d, want 2", got)
	}

	wantPhases := []string{
		ArchiveUploadPhaseExtracting,
		ArchiveUploadPhaseExtracting,
		ArchiveUploadPhaseExtracting,
		ArchiveUploadPhaseUploading,
		ArchiveUploadPhaseUploading,
		ArchiveUploadPhaseCreating,
	}
	if got := len(progressEvents); got != len(wantPhases) {
		t.Fatalf("progress events = %d, want %d", got, len(wantPhases))
	}
	for i, phase := range wantPhases {
		if progressEvents[i].Phase != phase {
			t.Fatalf("progress event %d phase = %q, want %q", i, progressEvents[i].Phase, phase)
		}
	}
	if progressEvents[0].Total != 2 {
		t.Fatalf("initial extract total = %d, want 2", progressEvents[0].Total)
	}
	if progressEvents[1].Current != 1 || progressEvents[1].FileName != "002-page.png" {
		t.Fatalf("first extracted event = %+v, want first extracted file", progressEvents[1])
	}
	if progressEvents[3].Current != 1 || progressEvents[3].FileName != "001-page.png" {
		t.Fatalf("first upload event = %+v, want sorted first upload file", progressEvents[3])
	}
}

func chapterImportPayload(provider string, mode string, url string) model.ChapterImportPayload {
	config, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		panic(err)
	}
	return model.ChapterImportPayload{
		Provider: provider,
		Mode:     mode,
		Config:   config,
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	cfg := config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "mangashelf-test.db"),
	}
	database, err := dbpkg.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.SQL.Close()
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(database.SQL, logger)
}

func newTestManga(t *testing.T, store *Store, ctx context.Context) *model.Manga {
	t.Helper()

	manga, err := store.CreateManga(ctx, model.MangaPayload{PrimaryTitle: "Test Manga"})
	if err != nil {
		t.Fatalf("CreateManga returned error: %v", err)
	}
	return manga
}

func hasSyncLog(logs []model.ChapterSourceSyncLog, inserted int, updated int, unchanged int, skipped int) bool {
	for _, log := range logs {
		if log.InsertedCount == inserted &&
			log.UpdatedCount == updated &&
			log.UnchangedCount == unchanged &&
			log.SkippedCount == skipped {
			return true
		}
	}
	return false
}

func hasInfoEntry(entries []model.InfoEntry, key string, value string) bool {
	for _, entry := range entries {
		if entry.Key == key && entry.Value == value {
			return true
		}
	}
	return false
}

type cubariFixture struct {
	mu     sync.RWMutex
	body   string
	server *httptest.Server
}

func newCubariFixture(t *testing.T, body string) *cubariFixture {
	t.Helper()

	fixture := &cubariFixture{body: body}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fixture.mu.RLock()
		defer fixture.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fixture.body)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *cubariFixture) URL() string {
	return f.server.URL
}

func (f *cubariFixture) SetBody(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body = body
}

func newArchiveUploadHeader(t *testing.T, files map[string]string) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "chapter.cbz")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}

	archiveBuffer := bytes.NewBuffer(nil)
	archiveWriter := zip.NewWriter(archiveBuffer)
	for name, content := range files {
		entry, err := archiveWriter.Create(name)
		if err != nil {
			t.Fatalf("zip Create returned error: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("zip entry write returned error: %v", err)
		}
	}
	if err := archiveWriter.Close(); err != nil {
		t.Fatalf("zip Close returned error: %v", err)
	}
	if _, err := part.Write(archiveBuffer.Bytes()); err != nil {
		t.Fatalf("multipart write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(body.Len()) + (1 << 20)); err != nil {
		t.Fatalf("ParseMultipartForm returned error: %v", err)
	}
	_, header, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile returned error: %v", err)
	}
	return header
}

func cubariPayload(chapters map[string][]string) string {
	payload := map[string]any{
		"title":    "Fixture Source",
		"chapters": map[string]any{},
	}

	chapterMap := payload["chapters"].(map[string]any)
	for number, pages := range chapters {
		chapterMap[number] = map[string]any{
			"title":        "Chapter " + number,
			"volume":       "1",
			"last_updated": 1714435200,
			"groups": map[string]any{
				"Default": pages,
			},
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}
