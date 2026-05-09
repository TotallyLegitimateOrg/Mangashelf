package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	extensionassets "github.com/TotallyLegitimateOrg/Mangashelf/extension"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/auth"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/buildinfo"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/model"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/services"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/store"
	webassets "github.com/TotallyLegitimateOrg/Mangashelf/web"
)

type Server struct {
	cfg           config.Config
	log           *slog.Logger
	store         *store.Store
	auth          *auth.Manager
	spaFS         fs.FS
	extensionFS   fs.FS
	versioningRaw []byte
	extensionURL  *url.URL
	httpClient    *http.Client
}

func New(cfg config.Config, logger *slog.Logger, users *store.Store, authManager *auth.Manager) (*Server, error) {
	var (
		spaFS         fs.FS
		extensionFS   fs.FS
		versioning    []byte
		extensionURL  *url.URL
		err           error
	)

	if cfg.DevWebURL == "" {
		spaFS, err = fs.Sub(webassets.Dist, "dist")
		if err != nil {
			return nil, fmt.Errorf("load embedded web assets: %w", err)
		}
	}

	if cfg.DevExtensionURL == "" {
		extensionFS, err = fs.Sub(extensionassets.Bundles, "bundles")
		if err != nil {
			return nil, fmt.Errorf("load embedded extension assets: %w", err)
		}
		versioning, err = fs.ReadFile(extensionFS, "versioning.json")
		if err != nil {
			return nil, err
		}
	} else {
		extensionURL, err = url.Parse(cfg.DevExtensionURL)
		if err != nil {
			return nil, fmt.Errorf("parse MANGASHELF_DEV_EXTENSION_URL: %w", err)
		}
	}

	return &Server{
		cfg:           cfg,
		log:           logger,
		store:         users,
		auth:          authManager,
		spaFS:         spaFS,
		extensionFS:   extensionFS,
		versioningRaw: versioning,
		extensionURL:  extensionURL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/buildinfo", s.handleBuildInfo)
	mux.HandleFunc("GET /api/auth/needs-setup", s.handleNeedsSetup)
	mux.HandleFunc("POST /api/auth/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.Handle("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.Handle("POST /api/auth/api-keys", s.requireAuth(s.handleCreateAPIKey))
	mux.Handle("GET /api/auth/api-keys", s.requireAuth(s.handleListAPIKeys))
	mux.Handle("DELETE /api/auth/api-keys/{id}", s.requireAuth(s.handleDeleteAPIKey))

	mux.Handle("GET /api/manga", s.requireAuth(s.handleListManga))
	mux.Handle("POST /api/manga", s.requireAuth(s.handleCreateManga))
	mux.Handle("GET /api/manga/{id}", s.requireAuth(s.handleGetManga))
	mux.Handle("PUT /api/manga/{id}", s.requireAuth(s.handleUpdateManga))
	mux.Handle("DELETE /api/manga/{id}", s.requireAuth(s.handleDeleteManga))

	mux.Handle("GET /api/collections", s.requireAuth(s.handleListCollections))
	mux.Handle("POST /api/collections", s.requireAuth(s.handleCreateCollection))
	mux.Handle("PATCH /api/collections/{id}", s.requireAuth(s.handleUpdateCollection))
	mux.Handle("DELETE /api/collections/{id}", s.requireAuth(s.handleDeleteCollection))
	mux.Handle("PUT /api/collections/reorder", s.requireAuth(s.handleReorderCollections))
	mux.Handle("GET /api/collections/{id}/manga", s.requireAuth(s.handleListCollectionManga))
	mux.Handle("PUT /api/collections/{id}/manga", s.requireAuth(s.handleReplaceCollectionManga))
	mux.Handle("POST /api/collections/{id}/changes", s.requireAuth(s.handleApplyCollectionChanges))

	mux.Handle("GET /api/manga/{id}/chapters", s.requireAuth(s.handleListChapters))
	mux.Handle("POST /api/manga/{id}/chapters", s.requireAuth(s.handleCreateChapter))
	mux.Handle("POST /api/manga/{id}/chapters/upload", s.requireAuth(s.handleUploadChapter))
	mux.Handle("POST /api/manga/{id}/chapter-imports", s.requireAuth(s.handleCreateChapterImport))
	mux.Handle("GET /api/manga/{id}/chapters/{chapterId}", s.requireAuth(s.handleGetChapter))
	mux.Handle("PUT /api/manga/{id}/chapters/{chapterId}", s.requireAuth(s.handleUpdateChapter))
	mux.Handle("DELETE /api/manga/{id}/chapters/{chapterId}", s.requireAuth(s.handleDeleteChapter))
	mux.Handle("PUT /api/manga/{id}/chapters/reorder", s.requireAuth(s.handleReorderChapters))

	mux.Handle("GET /api/manga/{id}/chapter-sources", s.requireAuth(s.handleListChapterSources))
	mux.Handle("POST /api/manga/{id}/chapter-sources/{sourceId}/sync", s.requireAuth(s.handleSyncChapterSource))
	mux.Handle("DELETE /api/manga/{id}/chapter-sources/{sourceId}/logs", s.requireAuth(s.handleClearChapterSourceLogs))
	mux.Handle("DELETE /api/manga/{id}/chapter-sources/{sourceId}", s.requireAuth(s.handleDeleteChapterSource))

	mux.Handle("GET /api/discover", s.requireAuth(s.handleListDiscover))
	mux.Handle("GET /api/discover/admin", s.requireAuth(s.handleListDiscoverAdmin))
	mux.Handle("POST /api/discover/admin", s.requireAuth(s.handleCreateDiscover))
	mux.Handle("PUT /api/discover/admin/reorder", s.requireAuth(s.handleReorderDiscover))
	mux.Handle("PUT /api/discover/admin/{id}", s.requireAuth(s.handleUpdateDiscover))
	mux.Handle("DELETE /api/discover/admin/{id}", s.requireAuth(s.handleDeleteDiscover))

	mux.Handle("GET /api/anilist/{id}", s.requireAuth(s.handleAniList))

	mux.HandleFunc("GET /extensions/paperback", s.handleExtensionVersioning)
	mux.HandleFunc("GET /extensions/paperback/", s.handleExtensionStatic)
	mux.HandleFunc("GET /extensions/paperback/versioning.json", s.handleExtensionVersioning)
	mux.HandleFunc("GET /extensions/paperback/download", s.handleExtensionDownload)

	mux.Handle("/", s.handleSPA())
	return s.withLogging(mux)
}

func (s *Server) requireAuth(next func(http.ResponseWriter, *http.Request, *store.UserIdentity)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.auth.AuthenticateToken(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			s.writeError(w, err)
			return
		}
		next(w, r, user)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	info := buildinfo.Current()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": nowUnixMillis(),
		"version":   info.Version,
		"commit":    info.Commit,
		"builtAt":   info.BuiltAt,
	})
}

func (s *Server) handleBuildInfo(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, buildinfo.Current())
}

func (s *Server) handleNeedsSetup(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.store.NeedsSetup(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": needsSetup})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	user, err := s.store.CreateInitialUser(r.Context(), payload.Username, payload.Password)
	if err != nil {
		s.writeError(w, err)
		return
	}
	token, err := s.auth.IssueToken(user)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	user, err := s.store.Login(r.Context(), payload.Username, payload.Password)
	if err != nil {
		s.writeError(w, err)
		return
	}
	token, err := s.auth.IssueToken(user)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, user *store.UserIdentity) {
	s.writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request, user *store.UserIdentity) {
	var payload struct {
		Name string `json:"name"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	apiKey, err := s.store.CreateAPIKey(r.Context(), user.ID, payload.Name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = "default"
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{"apiKey": apiKey, "name": name})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request, user *store.UserIdentity) {
	keys, err := s.store.ListAPIKeys(r.Context(), user.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"apiKeys": keys})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request, user *store.UserIdentity) {
	if err := s.store.DeleteAPIKey(r.Context(), user.ID, r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleListManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	options := parseMangaSearchOptions(r)
	items, err := s.store.SearchManga(r.Context(), options)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"manga": items})
}

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	items, err := s.store.ListCollections(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"collections": items})
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.CollectionPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.CreateCollection(r.Context(), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleUpdateCollection(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.CollectionPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.UpdateCollection(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteCollection(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.DeleteCollection(r.Context(), r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleListCollectionManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	items, err := s.store.ListCollectionManga(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"manga": items})
}

func (s *Server) handleReplaceCollectionManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.CollectionMangaPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	if err := s.store.ReplaceCollectionManga(r.Context(), r.PathValue("id"), payload); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleApplyCollectionChanges(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.CollectionChangesPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	if err := s.store.ApplyCollectionChanges(r.Context(), r.PathValue("id"), payload); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleReorderCollections(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload struct {
		Order []string `json:"order"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	if err := s.store.ReorderCollections(r.Context(), payload.Order); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleGetManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	item, err := s.store.GetManga(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.MangaPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.CreateManga(r.Context(), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleUpdateManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.MangaPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.UpdateManga(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteManga(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.DeleteManga(r.Context(), r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleListChapters(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	items, err := s.store.ListChapters(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"chapters": items})
}

func (s *Server) handleGetChapter(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	item, err := s.store.GetChapter(r.Context(), r.PathValue("id"), r.PathValue("chapterId"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateChapter(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.ChapterPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.CreateChapter(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleUpdateChapter(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.ChapterPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	item, err := s.store.UpdateChapter(r.Context(), r.PathValue("id"), r.PathValue("chapterId"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteChapter(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.DeleteChapter(r.Context(), r.PathValue("id"), r.PathValue("chapterId")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleReorderChapters(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload struct {
		Order []string `json:"order"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	if err := s.store.ReorderChapters(r.Context(), r.PathValue("id"), payload.Order); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleUploadChapter(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		s.writeError(w, fmt.Errorf("%w: invalid multipart form", store.ErrValidation))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, fmt.Errorf("%w: missing file", store.ErrValidation))
		return
	}
	_ = file.Close()
	payload := model.ChapterPayload{
		LangCode:       r.FormValue("langCode"),
		ChapNum:        parseFloat(r.FormValue("chapNum")),
		Title:          r.FormValue("title"),
		Version:        r.FormValue("version"),
		Volume:         parseOptionalFloatPtr(r.FormValue("volume")),
		PublishDate:    parseOptionalStringPtr(r.FormValue("publishDate")),
		CreationDate:   parseOptionalStringPtr(r.FormValue("creationDate")),
		SortingIndex:   parseOptionalFloatPtr(r.FormValue("sortingIndex")),
		AdditionalInfo: parseInfoEntriesJSON(r.FormValue("additionalInfo")),
	}
	if wantsUploadProgressStream(r) {
		if err := s.streamUploadChapter(w, r, header, payload); err != nil {
			s.log.Warn("stream upload failed", "err", err)
		}
		return
	}
	item, err := s.store.CreateChapterFromArchive(r.Context(), r.PathValue("id"), header, payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, item)
}

type uploadChapterStreamEvent struct {
	Type     string               `json:"type"`
	Phase    string               `json:"phase,omitempty"`
	Message  string               `json:"message,omitempty"`
	Current  int                  `json:"current,omitempty"`
	Total    int                  `json:"total,omitempty"`
	FileName string               `json:"fileName,omitempty"`
	Error    string               `json:"error,omitempty"`
	Chapter  *model.ChapterDetail `json:"chapter,omitempty"`
}

func wantsUploadProgressStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/x-ndjson") || r.Header.Get("X-Upload-Progress") == "1"
}

func (s *Server) streamUploadChapter(w http.ResponseWriter, r *http.Request, header *multipart.FileHeader, payload model.ChapterPayload) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		item, err := s.store.CreateChapterFromArchive(r.Context(), r.PathValue("id"), header, payload)
		if err != nil {
			s.writeError(w, err)
			return err
		}
		s.writeJSON(w, http.StatusCreated, item)
		return nil
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	encoder := json.NewEncoder(w)
	writeEvent := func(event uploadChapterStreamEvent) error {
		if err := encoder.Encode(event); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	item, err := s.store.CreateChapterFromArchiveWithProgress(r.Context(), r.PathValue("id"), header, payload, func(progress store.ArchiveUploadProgress) {
		if err := writeEvent(uploadChapterStreamEvent{
			Type:     "progress",
			Phase:    progress.Phase,
			Message:  progress.Message,
			Current:  progress.Current,
			Total:    progress.Total,
			FileName: progress.FileName,
		}); err != nil {
			s.log.Warn("writing upload progress failed", "err", err)
		}
	})
	if err != nil {
		if writeErr := writeEvent(uploadChapterStreamEvent{
			Type:  "error",
			Error: err.Error(),
		}); writeErr != nil {
			return writeErr
		}
		return nil
	}

	return writeEvent(uploadChapterStreamEvent{
		Type:    "complete",
		Chapter: item,
	})
}

func (s *Server) handleCreateChapterImport(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.ChapterImportPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.store.CreateChapterImport(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListChapterSources(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	sources, err := s.store.ListChapterSources(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	syncLogs, err := s.store.ListChapterSourceSyncLogs(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"sources":  sources,
		"syncLogs": syncLogs,
	})
}

func (s *Server) handleSyncChapterSource(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	source, stats, err := s.store.SyncChapterSource(r.Context(), r.PathValue("id"), r.PathValue("sourceId"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"source":         source,
		"insertedCount":  stats.Inserted,
		"updatedCount":   stats.Updated,
		"unchangedCount": stats.Unchanged,
		"skippedCount":   stats.Skipped,
	})
}

func (s *Server) handleDeleteChapterSource(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.UnlinkChapterSource(r.Context(), r.PathValue("id"), r.PathValue("sourceId")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleClearChapterSourceLogs(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.ClearChapterSourceSyncLogs(r.Context(), r.PathValue("id"), r.PathValue("sourceId")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleListDiscover(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	sections, err := s.store.ListDiscoverSections(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"sections": sections})
}

func (s *Server) handleListDiscoverAdmin(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	sections, err := s.store.ListDiscoverSectionConfigs(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"sections": sections})
}

func (s *Server) handleCreateDiscover(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.DiscoverSectionPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	section, err := s.store.CreateDiscoverSection(r.Context(), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, section)
}

func (s *Server) handleUpdateDiscover(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload model.DiscoverSectionPayload
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	section, err := s.store.UpdateDiscoverSection(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, section)
}

func (s *Server) handleDeleteDiscover(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	if err := s.store.DeleteDiscoverSection(r.Context(), r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleReorderDiscover(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	var payload struct {
		Order []string `json:"order"`
	}
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	if err := s.store.ReorderDiscoverSections(r.Context(), payload.Order); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleAniList(w http.ResponseWriter, r *http.Request, _ *store.UserIdentity) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		s.writeError(w, fmt.Errorf("%w: AniList ID must be a positive integer", store.ErrValidation))
		return
	}
	payload, err := services.FetchAniListManga(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleExtensionVersioning(w http.ResponseWriter, r *http.Request) {
	if s.extensionURL != nil {
		s.proxyDevExtension(w, r, "/versioning.json", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.versioningRaw)
}

func (s *Server) handleExtensionDownload(w http.ResponseWriter, r *http.Request) {
	if s.extensionURL != nil {
		s.proxyDevExtension(w, r, "/Mangashelf/index.js", `attachment; filename="Mangashelf.paperback.js"`)
		return
	}
	data, err := fs.ReadFile(s.extensionFS, "Mangashelf/index.js")
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Content-Disposition", `attachment; filename="Mangashelf.paperback.js"`)
	_, _ = w.Write(data)
}

func (s *Server) handleExtensionStatic(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/extensions/paperback/")
	if cleanPath == "" {
		s.handleExtensionVersioning(w, r)
		return
	}
	if s.extensionURL != nil {
		s.proxyDevExtension(w, r, "/"+cleanPath, "")
		return
	}
	data, err := fs.ReadFile(s.extensionFS, cleanPath)
	if err != nil {
		s.writeError(w, err)
		return
	}
	http.ServeContent(w, r, path.Base(cleanPath), time.Time{}, strings.NewReader(string(data)))
}

func (s *Server) handleSPA() http.Handler {
	var fileServer http.Handler
	if s.spaFS != nil {
		fileServer = http.FileServer(http.FS(s.spaFS))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/extensions/") {
			http.NotFound(w, r)
			return
		}
		if s.cfg.DevWebURL != "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			target := s.devWebRedirectTarget(r)
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
			return
		}
		if s.spaFS == nil || fileServer == nil {
			s.writeError(w, fmt.Errorf("web assets are unavailable; run a release build or set MANGASHELF_DEV_WEB_URL"))
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}
		if _, err := fs.Stat(s.spaFS, cleanPath); err == nil && !strings.HasSuffix(r.URL.Path, "/") {
			fileServer.ServeHTTP(w, r)
			return
		}

		index, err := fs.ReadFile(s.spaFS, "index.html")
		if err != nil {
			s.writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func (s *Server) proxyDevExtension(
	w http.ResponseWriter,
	r *http.Request,
	targetPath string,
	contentDisposition string,
) {
	if s.extensionURL == nil {
		http.NotFound(w, r)
		return
	}

	method := http.MethodGet
	var rawQuery string
	var headers http.Header
	if r != nil {
		method = r.Method
		rawQuery = r.URL.RawQuery
		headers = r.Header.Clone()
	} else {
		headers = make(http.Header)
	}

	target := *s.extensionURL
	target.Path = joinURLPath(s.extensionURL.Path, targetPath)
	target.RawQuery = rawQuery

	req, err := http.NewRequestWithContext(contextOrBackground(r), method, target.String(), nil)
	if err != nil {
		s.writeError(w, err)
		return
	}
	req.Header = headers

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if contentDisposition != "" {
		w.Header().Set("Content-Disposition", contentDisposition)
	}
	w.WriteHeader(resp.StatusCode)
	if method != http.MethodHead {
		_, _ = io.Copy(w, resp.Body)
	}
}

func contextOrBackground(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}

func joinURLPath(base string, suffix string) string {
	base = strings.TrimSuffix(base, "/")
	suffix = "/" + strings.TrimPrefix(suffix, "/")
	if base == "" {
		return suffix
	}
	return base + suffix
}

func (s *Server) devWebRedirectTarget(r *http.Request) string {
	target, err := url.Parse(s.cfg.DevWebURL)
	if err != nil {
		return s.cfg.DevWebURL + r.URL.RequestURI()
	}

	if requestHost := requestHostname(r.Host); requestHost != "" {
		target.Host = net.JoinHostPort(requestHost, target.Port())
	}
	target.Path = r.URL.Path
	target.RawPath = r.URL.RawPath
	target.RawQuery = r.URL.RawQuery
	target.Fragment = r.URL.Fragment
	return target.String()
}

func requestHostname(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}

	if strings.Contains(err.Error(), "missing port in address") {
		return hostport
	}

	return hostport
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		s.writeError(w, fmt.Errorf("%w: invalid JSON body", store.ErrValidation))
		return false
	}
	return true
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Internal server error"

	switch {
	case errors.Is(err, store.ErrUnauthorized):
		status = http.StatusUnauthorized
		message = "Invalid or expired token"
	case errors.Is(err, store.ErrForbidden):
		status = http.StatusForbidden
		message = err.Error()
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
		message = "Resource not found"
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrSetupAlreadyExists):
		status = http.StatusConflict
		message = err.Error()
	case errors.Is(err, store.ErrValidation):
		status = http.StatusBadRequest
		message = err.Error()
	default:
		if err != nil {
			message = err.Error()
		}
	}

	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func parseOptionalFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func parseOptionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func parseInfoEntriesJSON(raw string) []model.InfoEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var entries []model.InfoEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil
	}
	return entries
}

func parseMangaSearchOptions(r *http.Request) model.MangaSearchOptions {
	query := r.URL.Query()
	return model.MangaSearchOptions{
		Query:         query.Get("q"),
		ContentRating: splitCSVQuery(query["contentRating"]),
		Status:        splitCSVQuery(query["status"]),
		Tags:          splitCSVQuery(query["tag"]),
		MinRating:     parseOptionalFloatPtr(query.Get("minRating")),
		MaxRating:     parseOptionalFloatPtr(query.Get("maxRating")),
		Sort:          query.Get("sort"),
	}
}

func splitCSVQuery(values []string) []string {
	result := []string{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func nowUnixMillis() int64 {
	return time.Now().UTC().UnixMilli()
}
