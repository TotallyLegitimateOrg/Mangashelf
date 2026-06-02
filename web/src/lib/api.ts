/* ============================================================
   API client — central fetch wrapper with JWT injection
   ============================================================ */
import type {
  Manga,
  MangaSearchOptions,
  MangaPayload,
  ChapterListItem,
  ChapterDetail,
  ChapterPayload,
  ChapterBulkMetadataPayload,
  ChapterBulkMetadataResult,
  ChapterSource,
  ChapterSourceSyncLog,
  ChapterImportPayload,
  ChapterImportResult,
  DiscoverSectionConfig,
  DiscoverSectionPayload,
  APIKey,
  ExtensionMetadata,
  BuildInfo,
  User,
  Collection,
  CollectionPayload,
  CollectionMangaPayload,
  CollectionChangesPayload,
  BackupRestoreResult,
} from "./types";

const TOKEN_KEY = "mangashelf_token";

export function getStoredToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
export function setStoredToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}
export function clearStoredToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  status: number;
  method?: string;
  path?: string;

  constructor(message: string, status: number, request?: { method?: string; path?: string }) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.method = request?.method;
    this.path = request?.path;
  }
}

export function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    const details = [
      err.status > 0 ? String(err.status) : null,
      err.method,
      err.path,
    ].filter(Boolean);
    return details.length > 0 ? `${err.message} (${details.join(" ")})` : err.message;
  }
  if (err instanceof Error && err.message.trim()) {
    return err.message;
  }
  return fallback;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  opts?: { raw?: boolean }
): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (body && !opts?.raw) headers["Content-Type"] = "application/json";

  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      body: opts?.raw ? (body as BodyInit) : body ? JSON.stringify(body) : undefined,
    });
  } catch (err) {
    const reason = err instanceof Error && err.message.trim() ? `: ${err.message}` : "";
    throw new ApiError(`Network request failed${reason}`, 0, { method, path });
  }

  if (!res.ok) {
    let message = res.statusText ? `Request failed: ${res.statusText}` : "Request failed";
    try {
      const err = (await res.json()) as { error?: string };
      if (err.error) message = err.error;
    } catch {
      /* ignore parse errors */
    }
    throw new ApiError(message, res.status, { method, path });
  }

  try {
    return await res.json() as T;
  } catch (err) {
    const reason = err instanceof Error && err.message.trim() ? `: ${err.message}` : "";
    throw new ApiError(`Response was not valid JSON${reason}`, res.status, { method, path });
  }
}

export type UploadChapterProgressPhase =
  | "uploading_archive"
  | "extracting_archive"
  | "uploading_images"
  | "creating_chapter";

export interface UploadChapterProgress {
  phase: UploadChapterProgressPhase;
  message?: string;
  loadedBytes?: number;
  totalBytes?: number;
  current?: number;
  total?: number;
  fileName?: string;
}

interface UploadChapterStreamProgressEvent {
  type: "progress";
  phase: Exclude<UploadChapterProgressPhase, "uploading_archive">;
  message?: string;
  current?: number;
  total?: number;
  fileName?: string;
}

interface UploadChapterStreamCompleteEvent {
  type: "complete";
  chapter: ChapterDetail;
}

interface UploadChapterStreamErrorEvent {
  type: "error";
  error: string;
}

type UploadChapterStreamEvent =
  | UploadChapterStreamProgressEvent
  | UploadChapterStreamCompleteEvent
  | UploadChapterStreamErrorEvent;

function parseApiErrorMessage(status: number, responseText: string): string {
  let message = `Request failed (${status})`;
  try {
    const err = JSON.parse(responseText) as { error?: string };
    if (err.error) {
      return err.error;
    }
  } catch {
    /* ignore parse errors */
  }
  const trimmed = responseText.trim();
  if (trimmed) {
    return trimmed;
  }
  return message;
}

/* ---- Auth ---- */

export async function checkNeedsSetup(): Promise<boolean> {
  const data = await request<{ needsSetup: boolean }>("GET", "/api/auth/needs-setup");
  return data.needsSetup;
}

export async function setup(
  username: string,
  password: string
): Promise<{ token: string; user: User }> {
  return request("POST", "/api/auth/setup", { username, password });
}

export async function login(
  username: string,
  password: string
): Promise<{ token: string; user: User }> {
  return request("POST", "/api/auth/login", { username, password });
}

export async function getMe(): Promise<User> {
  const data = await request<{ user: User }>("GET", "/api/auth/me");
  return data.user;
}

/* ---- API Keys ---- */

export async function listAPIKeys(): Promise<APIKey[]> {
  const data = await request<{ apiKeys: APIKey[] }>("GET", "/api/auth/api-keys");
  return data.apiKeys;
}

export async function createAPIKey(
  name: string
): Promise<{ apiKey: string; name: string }> {
  return request("POST", "/api/auth/api-keys", { name });
}

export async function deleteAPIKey(id: string): Promise<void> {
  await request("DELETE", `/api/auth/api-keys/${id}`);
}

/* ---- Extension Repository ---- */

export async function getExtensionMetadata(): Promise<ExtensionMetadata> {
  return request<ExtensionMetadata>("GET", "/extensions/paperback/versioning.json");
}

export async function getBuildInfo(): Promise<BuildInfo> {
  return request<BuildInfo>("GET", "/api/buildinfo");
}

/* ---- Backups ---- */

export async function downloadBackup(): Promise<void> {
  const headers: Record<string, string> = {};
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;

  let res: Response;
  try {
    res = await fetch("/api/backups/export", { method: "GET", headers });
  } catch (err) {
    const reason = err instanceof Error && err.message.trim() ? `: ${err.message}` : "";
    throw new ApiError(`Network request failed${reason}`, 0, {
      method: "GET",
      path: "/api/backups/export",
    });
  }
  if (!res.ok) {
    const text = await res.text();
    throw new ApiError(parseApiErrorMessage(res.status, text), res.status, {
      method: "GET",
      path: "/api/backups/export",
    });
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `mangashelf-backup-${new Date().toISOString().slice(0, 10)}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export async function restoreBackup(file: File): Promise<BackupRestoreResult> {
  let payload: unknown;
  try {
    payload = JSON.parse(await file.text());
  } catch {
    throw new Error("Backup file must contain valid JSON");
  }
  return request<BackupRestoreResult>("POST", "/api/backups/restore", payload);
}

/* ---- Manga ---- */

export async function listManga(query?: string | MangaSearchOptions): Promise<Manga[]> {
  const options: MangaSearchOptions = typeof query === "string" ? { q: query } : query ?? {};
  const params = new URLSearchParams();
  if (options.q) params.set("q", options.q);
  for (const rating of options.contentRating ?? []) params.append("contentRating", rating);
  for (const status of options.status ?? []) params.append("status", status);
  for (const tag of options.tag ?? []) params.append("tag", tag);
  if (options.minRating != null) params.set("minRating", String(options.minRating));
  if (options.maxRating != null) params.set("maxRating", String(options.maxRating));
  if (options.sort) params.set("sort", options.sort);
  const qs = params.size > 0 ? `?${params.toString()}` : "";
  const data = await request<{ manga: Manga[] }>("GET", `/api/manga${qs}`);
  return data.manga;
}

export async function getManga(id: string): Promise<Manga> {
  return request<Manga>("GET", `/api/manga/${id}`);
}

export async function createManga(payload: MangaPayload): Promise<Manga> {
  return request<Manga>("POST", "/api/manga", payload);
}

export async function updateManga(
  id: string,
  payload: MangaPayload
): Promise<Manga> {
  return request<Manga>("PUT", `/api/manga/${id}`, payload);
}

export async function deleteManga(id: string): Promise<void> {
  await request("DELETE", `/api/manga/${id}`);
}

/* ---- Collections ---- */

export async function listCollections(): Promise<Collection[]> {
  const data = await request<{ collections: Collection[] }>("GET", "/api/collections");
  return data.collections;
}

export async function createCollection(payload: CollectionPayload): Promise<Collection> {
  return request<Collection>("POST", "/api/collections", payload);
}

export async function updateCollection(id: string, payload: CollectionPayload): Promise<Collection> {
  return request<Collection>("PATCH", `/api/collections/${id}`, payload);
}

export async function deleteCollection(id: string): Promise<void> {
  await request("DELETE", `/api/collections/${id}`);
}

export async function listCollectionManga(id: string): Promise<Manga[]> {
  const data = await request<{ manga: Manga[] }>("GET", `/api/collections/${id}/manga`);
  return data.manga;
}

export async function replaceCollectionManga(
  id: string,
  payload: CollectionMangaPayload
): Promise<void> {
  await request("PUT", `/api/collections/${id}/manga`, payload);
}

export async function applyCollectionChanges(
  id: string,
  payload: CollectionChangesPayload
): Promise<void> {
  await request("POST", `/api/collections/${id}/changes`, payload);
}

export async function reorderCollections(order: string[]): Promise<void> {
  await request("PUT", "/api/collections/reorder", { order });
}

/* ---- Chapters ---- */

export async function listChapters(mangaId: string): Promise<ChapterListItem[]> {
  const data = await request<{ chapters: ChapterListItem[] }>(
    "GET",
    `/api/manga/${mangaId}/chapters`
  );
  return data.chapters;
}

export async function getChapter(
  mangaId: string,
  chapterId: string
): Promise<ChapterDetail> {
  return request<ChapterDetail>(
    "GET",
    `/api/manga/${mangaId}/chapters/${chapterId}`
  );
}

export async function createChapter(
  mangaId: string,
  payload: ChapterPayload
): Promise<ChapterDetail> {
  return request<ChapterDetail>(
    "POST",
    `/api/manga/${mangaId}/chapters`,
    payload
  );
}

export async function updateChapter(
  mangaId: string,
  chapterId: string,
  payload: ChapterPayload
): Promise<ChapterDetail> {
  return request<ChapterDetail>(
    "PUT",
    `/api/manga/${mangaId}/chapters/${chapterId}`,
    payload
  );
}

export async function bulkUpdateChapterMetadata(
  mangaId: string,
  payload: ChapterBulkMetadataPayload
): Promise<ChapterBulkMetadataResult> {
  return request<ChapterBulkMetadataResult>(
    "PATCH",
    `/api/manga/${mangaId}/chapters/bulk-metadata`,
    payload
  );
}

export async function deleteChapter(
  mangaId: string,
  chapterId: string
): Promise<void> {
  await request("DELETE", `/api/manga/${mangaId}/chapters/${chapterId}`);
}

export async function uploadChapter(
  mangaId: string,
  formData: FormData,
  opts?: { onProgress?: (progress: UploadChapterProgress) => void }
): Promise<ChapterDetail> {
  return new Promise<ChapterDetail>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const token = getStoredToken();
    let responseCursor = 0;
    let responseBuffer = "";
    let completedChapter: ChapterDetail | null = null;
    let streamedError: string | null = null;

    const emitProgress = (progress: UploadChapterProgress) => {
      opts?.onProgress?.(progress);
    };

    const handleStreamEvent = (event: UploadChapterStreamEvent) => {
      if (event.type === "progress") {
        emitProgress({
          phase: event.phase,
          message: event.message,
          current: event.current,
          total: event.total,
          fileName: event.fileName,
        });
        return;
      }
      if (event.type === "complete") {
        completedChapter = event.chapter;
        return;
      }
      streamedError = event.error;
    };

    const processResponseChunk = (chunk: string) => {
      responseBuffer += chunk;
      let newlineIndex = responseBuffer.indexOf("\n");
      while (newlineIndex !== -1) {
        const line = responseBuffer.slice(0, newlineIndex).trim();
        responseBuffer = responseBuffer.slice(newlineIndex + 1);
        if (line) {
          try {
            handleStreamEvent(JSON.parse(line) as UploadChapterStreamEvent);
          } catch {
            /* ignore non-stream responses until completion */
          }
        }
        newlineIndex = responseBuffer.indexOf("\n");
      }
    };

    xhr.open("POST", `/api/manga/${mangaId}/chapters/upload`);
    xhr.responseType = "text";
    xhr.setRequestHeader("Accept", "application/x-ndjson");
    xhr.setRequestHeader("X-Upload-Progress", "1");
    if (token) {
      xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    }

    xhr.upload.onprogress = (event) => {
      emitProgress({
        phase: "uploading_archive",
        loadedBytes: event.loaded,
        totalBytes: event.lengthComputable ? event.total : undefined,
      });
    };

    xhr.upload.onload = () => {
      emitProgress({
        phase: "extracting_archive",
        message: "Archive uploaded",
      });
    };

    xhr.onprogress = () => {
      const chunk = xhr.responseText.slice(responseCursor);
      responseCursor = xhr.responseText.length;
      processResponseChunk(chunk);
    };

    xhr.onerror = () => {
      reject(new ApiError("Network request failed", xhr.status || 0, {
        method: "POST",
        path: `/api/manga/${mangaId}/chapters/upload`,
      }));
    };

    xhr.onload = () => {
      const chunk = xhr.responseText.slice(responseCursor);
      responseCursor = xhr.responseText.length;
      processResponseChunk(chunk);

      const trailing = responseBuffer.trim();
      if (trailing) {
        try {
          handleStreamEvent(JSON.parse(trailing) as UploadChapterStreamEvent);
          responseBuffer = "";
        } catch {
          /* leave non-stream payloads for the fallback parser below */
        }
      }

      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new ApiError(parseApiErrorMessage(xhr.status, xhr.responseText), xhr.status, {
          method: "POST",
          path: `/api/manga/${mangaId}/chapters/upload`,
        }));
        return;
      }
      if (streamedError) {
        reject(new ApiError(streamedError, xhr.status, {
          method: "POST",
          path: `/api/manga/${mangaId}/chapters/upload`,
        }));
        return;
      }
      if (completedChapter) {
        resolve(completedChapter);
        return;
      }

      try {
        resolve(JSON.parse(xhr.responseText) as ChapterDetail);
      } catch {
        reject(new ApiError("Upload completed without a chapter response", xhr.status, {
          method: "POST",
          path: `/api/manga/${mangaId}/chapters/upload`,
        }));
      }
    };

    xhr.send(formData);
  });
}

export async function reorderChapters(
  mangaId: string,
  order: string[]
): Promise<void> {
  await request("PUT", `/api/manga/${mangaId}/chapters/reorder`, { order });
}

/* ---- Chapter Imports (generic provider API) ---- */

export async function createChapterImport(
  mangaId: string,
  payload: ChapterImportPayload
): Promise<ChapterImportResult> {
  return request<ChapterImportResult>(
    "POST",
    `/api/manga/${mangaId}/chapter-imports`,
    payload
  );
}

/* ---- Chapter Sources (management only) ---- */

export async function listChapterSources(
  mangaId: string
): Promise<{ sources: ChapterSource[]; syncLogs: ChapterSourceSyncLog[] }> {
  return request<{ sources: ChapterSource[]; syncLogs: ChapterSourceSyncLog[] }>(
    "GET",
    `/api/manga/${mangaId}/chapter-sources`
  );
}

export async function triggerSync(
  mangaId: string,
  sourceId: string
): Promise<{
  source: ChapterSource;
  insertedCount: number;
  updatedCount: number;
  unchangedCount: number;
  skippedCount: number;
}> {
  return request(
    "POST",
    `/api/manga/${mangaId}/chapter-sources/${sourceId}/sync`
  );
}

export async function unlinkChapterSource(
  mangaId: string,
  sourceId: string
): Promise<void> {
  await request(
    "DELETE",
    `/api/manga/${mangaId}/chapter-sources/${sourceId}`
  );
}

export async function clearChapterSourceLogs(
  mangaId: string,
  sourceId: string
): Promise<void> {
  await request(
    "DELETE",
    `/api/manga/${mangaId}/chapter-sources/${sourceId}/logs`
  );
}

/* ---- Discover Sections ---- */

export async function listDiscoverSections(): Promise<DiscoverSectionConfig[]> {
  const data = await request<{ sections: DiscoverSectionConfig[] }>(
    "GET",
    "/api/discover/admin"
  );
  return data.sections;
}

export async function createDiscoverSection(
  payload: DiscoverSectionPayload
): Promise<DiscoverSectionConfig> {
  return request<DiscoverSectionConfig>("POST", "/api/discover/admin", payload);
}

export async function updateDiscoverSection(
  id: string,
  payload: DiscoverSectionPayload
): Promise<DiscoverSectionConfig> {
  return request<DiscoverSectionConfig>("PUT", `/api/discover/admin/${id}`, payload);
}

export async function deleteDiscoverSection(id: string): Promise<void> {
  await request("DELETE", `/api/discover/admin/${id}`);
}

export async function reorderDiscoverSections(
  order: string[]
): Promise<void> {
  await request("PUT", "/api/discover/admin/reorder", { order });
}

/* ---- AniList ---- */

export async function fetchAniList(id: number): Promise<MangaPayload> {
  return request<MangaPayload>("GET", `/api/anilist/${id}`);
}
