-- +goose Up
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE api_keys (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  key_prefix TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys(key_hash);

CREATE TABLE manga (
  id TEXT PRIMARY KEY,
  primary_title TEXT NOT NULL,
  synopsis TEXT NOT NULL DEFAULT '',
  thumbnail_url TEXT NOT NULL DEFAULT '',
  banner_url TEXT NOT NULL DEFAULT '',
  content_rating TEXT NOT NULL DEFAULT 'SAFE',
  status TEXT NOT NULL DEFAULT 'Ongoing',
  artist TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  rating REAL,
  share_url TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE manga_titles (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  title_type TEXT NOT NULL DEFAULT 'secondary',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_manga_titles_manga_id ON manga_titles(manga_id, sort_order);

CREATE TABLE manga_artwork (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  image_url TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_manga_artwork_manga_id ON manga_artwork(manga_id, sort_order);

CREATE TABLE manga_tag_groups (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_manga_tag_groups_manga_id ON manga_tag_groups(manga_id, sort_order);

CREATE TABLE manga_tags (
  id TEXT PRIMARY KEY,
  tag_group_id TEXT NOT NULL REFERENCES manga_tag_groups(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_manga_tags_group_id ON manga_tags(tag_group_id, sort_order);

CREATE TABLE manga_info_entries (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  info_key TEXT NOT NULL,
  info_value TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_manga_info_entries_manga_id ON manga_info_entries(manga_id, sort_order);

CREATE TABLE chapters (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  lang_code TEXT NOT NULL DEFAULT 'EN',
  chap_num REAL NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  volume REAL,
  publish_date INTEGER,
  creation_date INTEGER,
  sorting_index REAL,
  origin_provider TEXT,
  origin_mode TEXT,
  origin_source_id TEXT,
  origin_source_status TEXT,
  origin_chapter_key TEXT,
  last_updated INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_chapters_identity
  ON chapters(manga_id, chap_num, lang_code, version);

CREATE INDEX idx_chapters_manga_id ON chapters(manga_id, chap_num, version);

CREATE TABLE chapter_pages (
  id TEXT PRIMARY KEY,
  chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
  page_num INTEGER NOT NULL,
  image_url TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_chapter_pages_identity
  ON chapter_pages(chapter_id, page_num);

CREATE TABLE chapter_info_entries (
  id TEXT PRIMARY KEY,
  chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
  info_key TEXT NOT NULL,
  info_value TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_chapter_info_entries_chapter_id
  ON chapter_info_entries(chapter_id, sort_order);

CREATE TABLE chapter_sources (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  mode TEXT NOT NULL,
  config_json TEXT NOT NULL CHECK (json_valid(config_json)),
  status TEXT NOT NULL DEFAULT 'ready',
  last_error TEXT NOT NULL DEFAULT '',
  last_seen_chapter_count INTEGER,
  sync_interval_minutes INTEGER NOT NULL DEFAULT 60,
  last_synced_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX idx_chapter_sources_manga_id
  ON chapter_sources(manga_id, created_at);

CREATE TABLE chapter_source_sync_logs (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES chapter_sources(id) ON DELETE CASCADE,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('success', 'error')),
  inserted_count INTEGER NOT NULL DEFAULT 0,
  updated_count INTEGER NOT NULL DEFAULT 0,
  unchanged_count INTEGER NOT NULL DEFAULT 0,
  skipped_count INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE INDEX idx_chapter_source_sync_logs_manga_id
  ON chapter_source_sync_logs(manga_id, created_at DESC);

CREATE TABLE collections (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX idx_collections_sort_order
  ON collections(sort_order, title);

CREATE TABLE collection_manga (
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  manga_id TEXT NOT NULL REFERENCES manga(id) ON DELETE CASCADE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (collection_id, manga_id)
);

CREATE INDEX idx_collection_manga_collection_id
  ON collection_manga(collection_id, sort_order);

CREATE TABLE discover_sections (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  subtitle TEXT NOT NULL DEFAULT '',
  section_type TEXT NOT NULL CHECK (
    section_type IN ('featured', 'simpleCarousel', 'prominentCarousel', 'chapterUpdates', 'genres')
  ),
  mode TEXT NOT NULL DEFAULT 'manual' CHECK (mode IN ('manual', 'live')),
  live_rule_json TEXT CHECK (live_rule_json IS NULL OR json_valid(live_rule_json)),
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0)
);

CREATE TABLE discover_items (
  id TEXT PRIMARY KEY,
  section_id TEXT NOT NULL REFERENCES discover_sections(id) ON DELETE CASCADE,
  item_type TEXT NOT NULL CHECK (
    item_type IN (
      'featuredCarouselItem',
      'simpleCarouselItem',
      'prominentCarouselItem',
      'chapterUpdatesCarouselItem',
      'genresCarouselItem'
    )
  ),
  sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
  manga_id TEXT,
  chapter_id TEXT,
  image_url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  subtitle TEXT NOT NULL DEFAULT '',
  supertitle TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  publish_date INTEGER,
  content_rating TEXT CHECK (content_rating IS NULL OR content_rating IN ('SAFE', 'MATURE', 'ADULT')),
  metadata_json TEXT CHECK (metadata_json IS NULL OR json_valid(metadata_json)),
  search_query_json TEXT CHECK (search_query_json IS NULL OR json_valid(search_query_json)),
  CHECK (
    (
      item_type = 'featuredCarouselItem'
      AND manga_id IS NOT NULL AND manga_id <> ''
      AND chapter_id IS NULL
      AND image_url <> ''
      AND title <> ''
      AND subtitle = ''
      AND name = ''
      AND publish_date IS NULL
      AND search_query_json IS NULL
    ) OR (
      item_type = 'simpleCarouselItem'
      AND manga_id IS NOT NULL AND manga_id <> ''
      AND chapter_id IS NULL
      AND image_url <> ''
      AND title <> ''
      AND supertitle = ''
      AND name = ''
      AND publish_date IS NULL
      AND search_query_json IS NULL
    ) OR (
      item_type = 'prominentCarouselItem'
      AND manga_id IS NOT NULL AND manga_id <> ''
      AND chapter_id IS NULL
      AND image_url <> ''
      AND title <> ''
      AND supertitle = ''
      AND name = ''
      AND publish_date IS NULL
      AND search_query_json IS NULL
    ) OR (
      item_type = 'chapterUpdatesCarouselItem'
      AND manga_id IS NOT NULL AND manga_id <> ''
      AND chapter_id IS NOT NULL AND chapter_id <> ''
      AND image_url <> ''
      AND title <> ''
      AND supertitle = ''
      AND name = ''
      AND search_query_json IS NULL
    ) OR (
      item_type = 'genresCarouselItem'
      AND manga_id IS NULL
      AND chapter_id IS NULL
      AND image_url = ''
      AND title = ''
      AND subtitle = ''
      AND supertitle = ''
      AND name <> ''
      AND publish_date IS NULL
      AND search_query_json IS NOT NULL
    )
  )
);

CREATE INDEX idx_discover_items_section_id
  ON discover_items(section_id, sort_order);

-- +goose Down
DROP TABLE IF EXISTS discover_items;
DROP TABLE IF EXISTS discover_sections;
DROP TABLE IF EXISTS collection_manga;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS chapter_source_sync_logs;
DROP TABLE IF EXISTS chapter_sources;
DROP TABLE IF EXISTS chapter_info_entries;
DROP TABLE IF EXISTS chapter_pages;
DROP TABLE IF EXISTS chapters;
DROP TABLE IF EXISTS manga_info_entries;
DROP TABLE IF EXISTS manga_tags;
DROP TABLE IF EXISTS manga_tag_groups;
DROP TABLE IF EXISTS manga_artwork;
DROP TABLE IF EXISTS manga_titles;
DROP TABLE IF EXISTS manga;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
