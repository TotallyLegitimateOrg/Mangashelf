-- name: SearchMangaSummaries :many
SELECT
  m.id,
  m.primary_title,
  m.synopsis,
  m.thumbnail_url,
  m.banner_url,
  m.content_rating,
  m.status,
  m.artist,
  m.author,
  m.rating,
  m.share_url,
  m.created_at,
  m.updated_at,
  COUNT(c.id) AS chapter_count
FROM manga m
LEFT JOIN chapters c ON c.manga_id = m.id
WHERE (
  sqlc.narg('query') IS NULL
  OR TRIM(sqlc.narg('query')) = ''
  OR LOWER(m.primary_title) LIKE '%' || LOWER(sqlc.narg('query')) || '%'
  OR EXISTS (
    SELECT 1
    FROM manga_titles mt
    WHERE mt.manga_id = m.id
      AND LOWER(mt.title) LIKE '%' || LOWER(sqlc.narg('query')) || '%'
  )
)
AND (
  sqlc.narg('content_ratings') IS NULL
  OR m.content_rating IN (SELECT value FROM json_each(sqlc.narg('content_ratings')))
)
AND (
  sqlc.narg('statuses') IS NULL
  OR m.status IN (SELECT value FROM json_each(sqlc.narg('statuses')))
)
AND (
  sqlc.narg('min_rating') IS NULL
  OR (m.rating IS NOT NULL AND m.rating >= sqlc.narg('min_rating'))
)
AND (
  sqlc.narg('max_rating') IS NULL
  OR (m.rating IS NOT NULL AND m.rating <= sqlc.narg('max_rating'))
)
AND (
  sqlc.narg('tags') IS NULL
  OR NOT EXISTS (
    SELECT 1
    FROM json_each(sqlc.narg('tags')) required_tag
    WHERE NOT EXISTS (
      SELECT 1
      FROM manga_tag_groups g
      JOIN manga_tags t ON t.tag_group_id = g.id
      WHERE g.manga_id = m.id
        AND (
          LOWER(t.title) = LOWER(required_tag.value)
          OR LOWER(g.title || ':' || t.title) = LOWER(required_tag.value)
      )
    )
  )
)
AND LOWER(sqlc.arg('sort')) = LOWER(sqlc.arg('sort'))
GROUP BY m.id
ORDER BY
  CASE WHEN ?7 = 'updated_asc' THEN m.updated_at END ASC,
  CASE WHEN ?7 = 'title_asc' THEN LOWER(m.primary_title) END ASC,
  CASE WHEN ?7 = 'title_desc' THEN LOWER(m.primary_title) END DESC,
  CASE WHEN ?7 = 'rating_desc' THEN m.rating END DESC,
  CASE WHEN ?7 = 'rating_asc' THEN m.rating END ASC,
  CASE WHEN ?7 = 'chapters_desc' THEN COUNT(c.id) END DESC,
  CASE WHEN ?7 = 'chapters_asc' THEN COUNT(c.id) END ASC,
  m.updated_at DESC,
  LOWER(m.primary_title) ASC,
  m.id ASC;

-- name: ListGenreTagCounts :many
SELECT
  g.title AS group_title,
  t.title AS tag_title,
  COUNT(DISTINCT g.manga_id) AS manga_count
FROM manga_tag_groups g
JOIN manga_tags t ON t.tag_group_id = g.id
GROUP BY g.title, t.title;

-- name: GetMangaByID :one
SELECT
  id,
  primary_title,
  synopsis,
  thumbnail_url,
  banner_url,
  content_rating,
  status,
  artist,
  author,
  rating,
  share_url,
  created_at,
  updated_at
FROM manga
WHERE id = ?;

-- name: CreateManga :exec
INSERT INTO manga (
  id,
  primary_title,
  synopsis,
  thumbnail_url,
  banner_url,
  content_rating,
  status,
  artist,
  author,
  rating,
  share_url,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateManga :exec
UPDATE manga
SET
  primary_title = ?,
  synopsis = ?,
  thumbnail_url = ?,
  banner_url = ?,
  content_rating = ?,
  status = ?,
  artist = ?,
  author = ?,
  rating = ?,
  share_url = ?,
  updated_at = ?
WHERE id = ?;

-- name: TouchMangaUpdatedAt :exec
UPDATE manga
SET updated_at = ?
WHERE id = ?;

-- name: DeleteManga :exec
DELETE FROM manga
WHERE id = ?;

-- name: DeleteMangaTitles :exec
DELETE FROM manga_titles
WHERE manga_id = ?;

-- name: InsertMangaTitle :exec
INSERT INTO manga_titles (id, manga_id, title, title_type, sort_order)
VALUES (?, ?, ?, ?, ?);

-- name: ListMangaTitles :many
SELECT id, manga_id, title, title_type, sort_order
FROM manga_titles
WHERE manga_id = ?
ORDER BY sort_order ASC, title ASC;

-- name: ListMangaTitlesByMangaIDs :many
SELECT id, manga_id, title, title_type, sort_order
FROM manga_titles
WHERE manga_id IN (sqlc.slice('manga_ids'))
ORDER BY manga_id ASC, sort_order ASC, title ASC;

-- name: DeleteMangaArtwork :exec
DELETE FROM manga_artwork
WHERE manga_id = ?;

-- name: InsertMangaArtwork :exec
INSERT INTO manga_artwork (id, manga_id, image_url, sort_order)
VALUES (?, ?, ?, ?);

-- name: ListMangaArtwork :many
SELECT id, manga_id, image_url, sort_order
FROM manga_artwork
WHERE manga_id = ?
ORDER BY sort_order ASC;

-- name: ListMangaArtworkByMangaIDs :many
SELECT id, manga_id, image_url, sort_order
FROM manga_artwork
WHERE manga_id IN (sqlc.slice('manga_ids'))
ORDER BY manga_id ASC, sort_order ASC;

-- name: DeleteMangaTagGroups :exec
DELETE FROM manga_tag_groups
WHERE manga_id = ?;

-- name: InsertMangaTagGroup :exec
INSERT INTO manga_tag_groups (id, manga_id, title, sort_order)
VALUES (?, ?, ?, ?);

-- name: InsertMangaTag :exec
INSERT INTO manga_tags (id, tag_group_id, title, sort_order)
VALUES (?, ?, ?, ?);

-- name: ListMangaTagsJoined :many
SELECT
  g.id AS group_id,
  g.title AS group_title,
  g.sort_order AS group_sort_order,
  t.id AS tag_id,
  t.title AS tag_title,
  t.sort_order AS tag_sort_order
FROM manga_tag_groups g
LEFT JOIN manga_tags t ON t.tag_group_id = g.id
WHERE g.manga_id = ?
ORDER BY g.sort_order ASC, t.sort_order ASC, t.title ASC;

-- name: ListMangaTagsJoinedByMangaIDs :many
SELECT
  g.id AS group_id,
  g.manga_id,
  g.title AS group_title,
  g.sort_order AS group_sort_order,
  t.id AS tag_id,
  t.title AS tag_title,
  t.sort_order AS tag_sort_order
FROM manga_tag_groups g
LEFT JOIN manga_tags t ON t.tag_group_id = g.id
WHERE g.manga_id IN (sqlc.slice('manga_ids'))
ORDER BY g.manga_id ASC, g.sort_order ASC, t.sort_order ASC, t.title ASC;

-- name: DeleteMangaInfoEntries :exec
DELETE FROM manga_info_entries
WHERE manga_id = ?;

-- name: InsertMangaInfoEntry :exec
INSERT INTO manga_info_entries (id, manga_id, info_key, info_value, sort_order)
VALUES (?, ?, ?, ?, ?);

-- name: ListMangaInfoEntries :many
SELECT id, manga_id, info_key, info_value, sort_order
FROM manga_info_entries
WHERE manga_id = ?
ORDER BY sort_order ASC;

-- name: ListMangaInfoEntriesByMangaIDs :many
SELECT id, manga_id, info_key, info_value, sort_order
FROM manga_info_entries
WHERE manga_id IN (sqlc.slice('manga_ids'))
ORDER BY manga_id ASC, sort_order ASC;
