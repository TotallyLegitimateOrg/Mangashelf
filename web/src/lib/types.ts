/* ============================================================
   TypeScript types mirroring the Go model layer
   ============================================================ */

export interface InfoEntry {
  key: string;
  value: string;
}

export interface Tag {
  id: string;
  title: string;
}

export interface TagGroup {
  id: string;
  title: string;
  tags: Tag[];
}

export interface SearchFilter {
  id: string;
  value: unknown;
}

export interface DiscoverSearchQuery {
  title: string;
  filters: SearchFilter[];
}

export interface DiscoverLiveRule {
  preset: string;
  limit: number;
}

export interface Manga {
  id: string;
  primaryTitle: string;
  secondaryTitles: string[];
  synopsis: string;
  thumbnailUrl: string;
  bannerUrl: string;
  contentRating: string;
  status: string;
  artist: string;
  author: string;
  rating: number | null;
  shareUrl: string;
  artworkUrls: string[];
  tagGroups: TagGroup[];
  additionalInfo: InfoEntry[];
  chapterCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface MangaPayload {
  primaryTitle: string;
  secondaryTitles: string[];
  synopsis: string;
  thumbnailUrl: string;
  bannerUrl: string;
  contentRating: string;
  status: string;
  artist: string;
  author: string;
  rating: number | null;
  shareUrl: string;
  artworkUrls: string[];
  tagGroups: TagGroup[];
  additionalInfo: InfoEntry[];
}

export interface MangaSearchOptions {
  q?: string;
  contentRating?: string[];
  status?: string[];
  tag?: string[];
  minRating?: number | null;
  maxRating?: number | null;
  sort?: string;
}

export interface Collection {
  id: string;
  title: string;
  sortOrder: number;
  mangaCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CollectionPayload {
  title: string;
}

export interface CollectionMangaPayload {
  mangaIds: string[];
}

export interface CollectionChangesPayload {
  additions: string[];
  deletions: string[];
}

export interface ChapterOrigin {
  kind: string;
  mode: string;
  readOnly: boolean;
  sourceId: string | null;
  sourceStatus: string | null;
  provider: string | null;
  chapterKey: string | null;
}

export interface ChapterListItem {
  id: string;
  mangaId: string;
  langCode: string;
  chapNum: number;
  title: string;
  version: string;
  volume: number | null;
  publishDate: string | null;
  creationDate: string | null;
  sortingIndex: number | null;
  additionalInfo: InfoEntry[];
  pageCount: number;
  lastUpdated: string;
  origin: ChapterOrigin;
}

export interface ChapterDetail extends ChapterListItem {
  pages: string[];
}

export interface ChapterPayload {
  langCode: string;
  chapNum: number;
  title: string;
  version: string;
  volume: number | null;
  publishDate: string | null;
  creationDate: string | null;
  sortingIndex: number | null;
  additionalInfo: InfoEntry[];
  pages: string[];
}

export interface ChapterSource {
  id: string;
  mangaId: string;
  provider: string;
  mode: string;
  config: Record<string, unknown>;
  status: string;
  lastError: string;
  lastSeenChapterCount: number | null;
  syncIntervalMinutes: number;
  lastSyncedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ChapterSourceSyncLog {
  id: string;
  sourceId: string;
  mangaId: string;
  status: "success" | "error";
  insertedCount: number;
  updatedCount: number;
  unchangedCount: number;
  skippedCount: number;
  error: string;
  createdAt: string;
}

/* ---- Chapter Import (generic provider API) ---- */

export interface ChapterImportPayload {
  provider: string;
  mode: "import_once" | "proxy" | "sync";
  config: Record<string, unknown>;
  syncIntervalMinutes?: number;
}

export interface ChapterImportResult {
  insertedCount: number;
  updatedCount: number;
  unchangedCount: number;
  skippedCount: number;
  sourceId: string | null;
  chapters: ChapterDetail[];
}

export interface BackupRestoreResult {
  mangaCount: number;
  chapterCount: number;
  collectionCount: number;
  discoverSectionCount: number;
  chapterSourceCount: number;
}

/* ---- Provider definitions ---- */

export interface ProviderDefinition {
  id: string;
  label: string;
  configFields: ProviderConfigField[];
  /** Build a human-readable summary of the config for display in source cards */
  configSummary: (config: Record<string, unknown>) => string;
}

export interface ProviderConfigField {
  key: string;
  label: string;
  placeholder: string;
  type: "text" | "url" | "number";
  required: boolean;
}

export const PROVIDERS: ProviderDefinition[] = [
  {
    id: "cubari",
    label: "Cubari",
    configFields: [
      {
        key: "url",
        label: "Cubari JSON URL",
        placeholder: "https://cubari.moe/read/api/gist/...",
        type: "url",
        required: true,
      },
    ],
    configSummary: (config) => (config.url as string) || "—",
  },
];

export const IMPORT_MODES = [
  { value: "import_once" as const, label: "Import Once", description: "Download all chapters now. No ongoing link." },
  { value: "proxy" as const, label: "Proxy", description: "Chapters are fetched live from the source on each request." },
  { value: "sync" as const, label: "Sync", description: "Import chapters now and keep in sync automatically." },
] as const;

export interface DiscoverSectionItem {
  id: string;
  type: string;
  mangaId?: string;
  chapterId?: string;
  chapNum?: number;
  imageUrl?: string;
  title?: string;
  subtitle?: string;
  supertitle?: string;
  name?: string;
  publishDate?: string | null;
  contentRating?: string | null;
  metadata?: unknown;
  searchQuery?: DiscoverSearchQuery | null;
}

export interface DiscoverSection {
  id: string;
  title: string;
  subtitle: string;
  type: string;
  sortOrder: number;
  items: DiscoverSectionItem[];
}

export interface DiscoverSectionConfig {
  id: string;
  title: string;
  subtitle: string;
  type: string;
  mode: string;
  liveRule?: DiscoverLiveRule | null;
  sortOrder: number;
  items: DiscoverSectionItem[];
}

export interface DiscoverSectionPayload {
  title: string;
  subtitle: string;
  type: string;
  mode: string;
  liveRule?: DiscoverLiveRule | null;
  items: DiscoverSectionItem[];
}

export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  createdAt: number;
}

export interface ExtensionMetadata {
  buildTime?: string;
  builtWith?: {
    toolchain?: string;
    types?: string;
  };
  repository?: {
    name?: string;
    description?: string;
  };
  sources: Array<{
    id?: string;
    name: string;
    description?: string;
    version: string;
    language?: string;
    contentRating?: string;
    capabilities?: number | string[];
    developers?: Array<{ name: string }>;
  }>;
}

export interface User {
  id: string;
  username: string;
}

export interface BuildInfo {
  version: string;
  commit: string;
  builtAt: string;
}

export const CONTENT_RATINGS = ["SAFE", "MATURE", "ADULT"] as const;
export const MANGA_STATUSES = ["Ongoing", "Completed", "Hiatus", "Cancelled"] as const;
export const DISCOVER_SECTION_TYPES = [
  "featured",
  "simpleCarousel",
  "prominentCarousel",
  "chapterUpdates",
  "genres",
] as const;

export function emptyMangaPayload(): MangaPayload {
  return {
    primaryTitle: "",
    secondaryTitles: [],
    synopsis: "",
    thumbnailUrl: "",
    bannerUrl: "",
    contentRating: "SAFE",
    status: "Ongoing",
    artist: "",
    author: "",
    rating: null,
    shareUrl: "",
    artworkUrls: [],
    tagGroups: [],
    additionalInfo: [],
  };
}

export function emptyChapterPayload(): ChapterPayload {
  return {
    langCode: "AR",
    chapNum: 0,
    title: "",
    version: "",
    volume: null,
    publishDate: null,
    creationDate: null,
    sortingIndex: null,
    additionalInfo: [],
    pages: [],
  };
}
