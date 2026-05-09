-- name: ListCollections :many
SELECT
  c.id,
  c.title,
  c.sort_order,
  c.created_at,
  c.updated_at,
  COUNT(cm.manga_id) AS manga_count
FROM collections c
LEFT JOIN collection_manga cm ON cm.collection_id = c.id
GROUP BY c.id
ORDER BY c.sort_order ASC, LOWER(c.title) ASC, c.id ASC;

-- name: GetCollectionByID :one
SELECT
  c.id,
  c.title,
  c.sort_order,
  c.created_at,
  c.updated_at,
  COUNT(cm.manga_id) AS manga_count
FROM collections c
LEFT JOIN collection_manga cm ON cm.collection_id = c.id
WHERE c.id = ?
GROUP BY c.id;

-- name: CreateCollection :exec
INSERT INTO collections (id, title, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateCollection :exec
UPDATE collections
SET title = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteCollection :exec
DELETE FROM collections
WHERE id = ?;

-- name: SetCollectionSortOrder :exec
UPDATE collections
SET sort_order = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteCollectionManga :exec
DELETE FROM collection_manga
WHERE collection_id = ?;

-- name: InsertCollectionManga :exec
INSERT INTO collection_manga (collection_id, manga_id, sort_order, created_at)
VALUES (?, ?, ?, ?);

-- name: AddMangaToCollection :exec
INSERT INTO collection_manga (collection_id, manga_id, sort_order, created_at)
VALUES (
  ?,
  ?,
  COALESCE((SELECT MAX(cm.sort_order) + 1 FROM collection_manga cm WHERE cm.collection_id = ?), 0),
  ?
)
ON CONFLICT(collection_id, manga_id) DO NOTHING;

-- name: RemoveMangaFromCollection :exec
DELETE FROM collection_manga
WHERE collection_id = ? AND manga_id = ?;

-- name: ListCollectionMangaSummaries :many
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
  COUNT(ch.id) AS chapter_count
FROM collection_manga cm
JOIN manga m ON m.id = cm.manga_id
LEFT JOIN chapters ch ON ch.manga_id = m.id
WHERE cm.collection_id = ?
GROUP BY m.id
ORDER BY cm.sort_order ASC, LOWER(m.primary_title) ASC, m.id ASC;
