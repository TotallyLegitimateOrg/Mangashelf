-- name: ListDiscoverSections :many
SELECT id, title, subtitle, section_type, sort_order, mode, live_rule_json
FROM discover_sections
ORDER BY sort_order ASC, title ASC;

-- name: ListDiscoverItems :many
SELECT
  id,
  section_id,
  item_type,
  sort_order,
  manga_id,
  chapter_id,
  image_url,
  title,
  subtitle,
  supertitle,
  name,
  publish_date,
  content_rating,
  metadata_json,
  search_query_json
FROM discover_items
ORDER BY section_id ASC, sort_order ASC, title ASC;

-- name: CreateDiscoverSection :exec
INSERT INTO discover_sections (id, title, subtitle, section_type, sort_order, mode, live_rule_json)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateDiscoverSection :exec
UPDATE discover_sections
SET
  title = ?,
  subtitle = ?,
  section_type = ?,
  mode = ?,
  live_rule_json = ?
WHERE id = ?;

-- name: DeleteDiscoverSection :exec
DELETE FROM discover_sections
WHERE id = ?;

-- name: DeleteDiscoverItemsBySectionID :exec
DELETE FROM discover_items
WHERE section_id = ?;

-- name: InsertDiscoverItem :exec
INSERT INTO discover_items (
  id,
  section_id,
  item_type,
  sort_order,
  manga_id,
  chapter_id,
  image_url,
  title,
  subtitle,
  supertitle,
  name,
  publish_date,
  content_rating,
  metadata_json,
  search_query_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetMaxDiscoverSortOrder :one
SELECT CAST(COALESCE(MAX(sort_order), -1) AS INTEGER)
FROM discover_sections;

-- name: UpdateDiscoverSectionSortOrder :exec
UPDATE discover_sections
SET sort_order = ?
WHERE id = ?;
