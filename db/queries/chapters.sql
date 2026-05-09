-- name: ListStoredChapters :many
SELECT
  c.id,
  c.manga_id,
  c.lang_code,
  c.chap_num,
  c.title,
  c.version,
  c.volume,
  c.publish_date,
  c.creation_date,
  c.sorting_index,
  c.origin_provider,
  c.origin_mode,
  c.origin_source_id,
  c.origin_source_status,
  c.origin_chapter_key,
  c.last_updated,
  COUNT(cp.id) AS page_count
FROM chapters c
LEFT JOIN chapter_pages cp ON cp.chapter_id = c.id
WHERE c.manga_id = ?
GROUP BY c.id
ORDER BY
  CASE WHEN c.sorting_index IS NULL THEN 1 ELSE 0 END ASC,
  c.sorting_index ASC,
  c.chap_num ASC,
  c.version ASC;

-- name: ListRecentStoredChapters :many
SELECT
  c.id,
  c.manga_id,
  c.title,
  c.chap_num,
  c.publish_date,
  c.creation_date,
  c.last_updated,
  m.primary_title,
  m.thumbnail_url
FROM chapters c
JOIN manga m ON m.id = c.manga_id
ORDER BY
  CASE WHEN c.publish_date IS NULL THEN 1 ELSE 0 END ASC,
  c.publish_date DESC,
  CASE WHEN c.creation_date IS NULL THEN 1 ELSE 0 END ASC,
  c.creation_date DESC,
  c.last_updated DESC,
  LOWER(m.primary_title) ASC,
  c.id ASC
LIMIT ?;

-- name: GetStoredChapterByID :one
SELECT
  c.id,
  c.manga_id,
  c.lang_code,
  c.chap_num,
  c.title,
  c.version,
  c.volume,
  c.publish_date,
  c.creation_date,
  c.sorting_index,
  c.origin_provider,
  c.origin_mode,
  c.origin_source_id,
  c.origin_source_status,
  c.origin_chapter_key,
  c.last_updated,
  COUNT(cp.id) AS page_count
FROM chapters c
LEFT JOIN chapter_pages cp ON cp.chapter_id = c.id
WHERE c.id = ?
GROUP BY c.id;

-- name: CreateChapter :exec
INSERT INTO chapters (
  id,
  manga_id,
  lang_code,
  chap_num,
  title,
  version,
  volume,
  publish_date,
  creation_date,
  sorting_index,
  origin_provider,
  origin_mode,
  origin_source_id,
  origin_source_status,
  origin_chapter_key,
  last_updated
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateChapter :exec
UPDATE chapters
SET
  lang_code = ?,
  chap_num = ?,
  title = ?,
  version = ?,
  volume = ?,
  publish_date = ?,
  creation_date = ?,
  sorting_index = ?,
  origin_provider = ?,
  origin_mode = ?,
  origin_source_id = ?,
  origin_source_status = ?,
  origin_chapter_key = ?,
  last_updated = ?
WHERE id = ?;

-- name: UpdateChapterSortingIndex :exec
UPDATE chapters
SET sorting_index = ?
WHERE id = ? AND manga_id = ?;

-- name: ClearChapterSortingIndices :exec
UPDATE chapters
SET sorting_index = NULL
WHERE manga_id = ?;

-- name: DeleteChapter :exec
DELETE FROM chapters
WHERE id = ?;

-- name: DeleteChapterPages :exec
DELETE FROM chapter_pages
WHERE chapter_id = ?;

-- name: InsertChapterPage :exec
INSERT INTO chapter_pages (id, chapter_id, page_num, image_url)
VALUES (?, ?, ?, ?);

-- name: ListChapterPages :many
SELECT id, chapter_id, page_num, image_url
FROM chapter_pages
WHERE chapter_id = ?
ORDER BY page_num ASC;

-- name: DeleteChapterInfoEntries :exec
DELETE FROM chapter_info_entries
WHERE chapter_id = ?;

-- name: InsertChapterInfoEntry :exec
INSERT INTO chapter_info_entries (id, chapter_id, info_key, info_value, sort_order)
VALUES (?, ?, ?, ?, ?);

-- name: ListChapterInfoEntries :many
SELECT id, chapter_id, info_key, info_value, sort_order
FROM chapter_info_entries
WHERE chapter_id = ?
ORDER BY sort_order ASC;

-- name: ListChapterSources :many
SELECT
  id,
  manga_id,
  provider,
  mode,
  config_json,
  status,
  last_error,
  last_seen_chapter_count,
  sync_interval_minutes,
  last_synced_at,
  created_at,
  updated_at
FROM chapter_sources
WHERE manga_id = ?
ORDER BY created_at DESC;

-- name: GetChapterSourceByID :one
SELECT
  id,
  manga_id,
  provider,
  mode,
  config_json,
  status,
  last_error,
  last_seen_chapter_count,
  sync_interval_minutes,
  last_synced_at,
  created_at,
  updated_at
FROM chapter_sources
WHERE id = ?;

-- name: FindChapterSourceByConfig :many
SELECT id
FROM chapter_sources
WHERE manga_id = ? AND provider = ? AND config_json = ?;

-- name: CreateChapterSource :exec
INSERT INTO chapter_sources (
  id,
  manga_id,
  provider,
  mode,
  config_json,
  status,
  last_error,
  last_seen_chapter_count,
  sync_interval_minutes,
  last_synced_at,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateChapterSource :exec
UPDATE chapter_sources
SET
  status = ?,
  last_error = ?,
  last_seen_chapter_count = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateChapterSourceSynced :exec
UPDATE chapter_sources
SET
  last_synced_at = ?,
  last_seen_chapter_count = ?,
  status = ?,
  last_error = ?,
  updated_at = ?
WHERE id = ?;

-- name: ListSyncDueSources :many
SELECT
  id,
  manga_id,
  provider,
  mode,
  config_json,
  status,
  last_error,
  last_seen_chapter_count,
  sync_interval_minutes,
  last_synced_at,
  created_at,
  updated_at
FROM chapter_sources
WHERE mode = 'sync'
  AND (last_synced_at IS NULL OR (? - last_synced_at) >= (sync_interval_minutes * 60))
ORDER BY last_synced_at ASC;

-- name: DeleteChapterSource :exec
DELETE FROM chapter_sources
WHERE id = ?;

-- name: CreateChapterSourceSyncLog :exec
INSERT INTO chapter_source_sync_logs (
  id,
  source_id,
  manga_id,
  status,
  inserted_count,
  updated_count,
  unchanged_count,
  skipped_count,
  error,
  created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListChapterSourceSyncLogs :many
SELECT
  id,
  source_id,
  manga_id,
  status,
  inserted_count,
  updated_count,
  unchanged_count,
  skipped_count,
  error,
  created_at
FROM chapter_source_sync_logs
WHERE manga_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: DeleteChapterSourceSyncLogs :exec
DELETE FROM chapter_source_sync_logs
WHERE manga_id = ? AND source_id = ?;

-- name: DeleteDiscoverItemsByChapterID :exec
DELETE FROM discover_items
WHERE chapter_id = ?;

-- name: DeleteDiscoverItemsByMangaID :exec
DELETE FROM discover_items
WHERE manga_id = ?;
