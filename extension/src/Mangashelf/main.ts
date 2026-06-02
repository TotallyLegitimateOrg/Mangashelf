/* SPDX-License-Identifier: GPL-3.0-or-later */

import {
  BasicRateLimiter,
  ContentRating,
  DiscoverSectionType,
  Form,
  type Chapter,
  type ChapterDetails,
  type DiscoverSection,
  type DiscoverSectionItem,
  type ExtensionImpl,
  type ManagedCollection,
  type ManagedCollectionChangeset,
  type Metadata,
  type PagedResults,
  type SearchQuery,
  type SearchResultItem,
  type SourceManga,
  type SortingOption,
} from "@paperback/types";
import {
  SearchFilterForm,
  type SearchFilter,
  type SearchFilterValue,
} from "@paperback/types/lib/compat/0.8";

import { SettingsForm } from "./forms";
import { MainInterceptor } from "./network";
import config from "./pbconfig";

interface InfoEntry {
  key: string;
  value: string;
}

interface Tag {
  id: string;
  title: string;
}

interface TagGroup {
  id: string;
  title: string;
  tags: Tag[];
}

interface SearchQueryValue {
  title: string;
  filters: SearchFilterValue[];
}

interface ApiManga {
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

interface ApiCollection {
  id: string;
  title: string;
  sortOrder: number;
  mangaCount: number;
  createdAt: string;
  updatedAt: string;
}

interface ApiChapter {
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
  pages?: string[];
}

type ApiDiscoverItem =
  | {
      id: string;
      type: "featuredCarouselItem";
      mangaId: string;
      imageUrl: string;
      title: string;
      supertitle: string;
      contentRating: string | null;
      metadata: unknown;
    }
  | {
      id: string;
      type: "simpleCarouselItem" | "prominentCarouselItem";
      mangaId: string;
      imageUrl: string;
      title: string;
      subtitle: string;
      contentRating: string | null;
      metadata: unknown;
    }
  | {
      id: string;
      type: "chapterUpdatesCarouselItem";
      mangaId: string;
      chapterId: string;
      chapNum: number;
      imageUrl: string;
      title: string;
      subtitle: string;
      publishDate: string | null;
      contentRating: string | null;
      metadata: unknown;
    }
  | {
      id: string;
      type: "genresCarouselItem";
      name: string;
      searchQuery: SearchQueryValue;
      contentRating: string | null;
      metadata: unknown;
    };

interface ApiDiscoverSection {
  id: string;
  title: string;
  subtitle: string;
  type: "featured" | "simpleCarousel" | "prominentCarousel" | "chapterUpdates" | "genres";
  sortOrder: number;
  items: ApiDiscoverItem[];
}

const SERVER_URL_STATE_KEY = "serverUrl";
const API_KEY_STATE_KEY = "apiKey";
const SORTING_OPTIONS = [
  { id: "updated_desc", label: "Recently Updated" },
  { id: "updated_asc", label: "Oldest Updated" },
  { id: "title_asc", label: "Title A-Z" },
  { id: "title_desc", label: "Title Z-A" },
  { id: "rating_desc", label: "Rating High-Low" },
  { id: "rating_asc", label: "Rating Low-High" },
  { id: "chapters_desc", label: "Most Chapters" },
  { id: "chapters_asc", label: "Fewest Chapters" },
] satisfies SortingOption[];

type ExtensionSearchQuery = SearchQuery<SearchFilterValue[]>;

function getIncludedFilterValues(value: SearchFilterValue["value"]): string[] {
  if (typeof value === "string") return value ? [value] : [];
  return Object.entries(value)
    .filter(([, state]) => state === "included")
    .map(([id]) => id);
}

function getStringFilterValue(value: SearchFilterValue["value"]): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized ? normalized : undefined;
}

function toContentRating(value: string | null | undefined): ContentRating | undefined {
  switch (value) {
    case "ADULT":
      return ContentRating.ADULT;
    case "MATURE":
      return ContentRating.MATURE;
    case "SAFE":
      return ContentRating.EVERYONE;
    default:
      return undefined;
  }
}

function toDate(value: string | null | undefined): Date | undefined {
  if (!value) return undefined;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed;
}

function toAdditionalInfo(entries: InfoEntry[]): Record<string, string> | undefined {
  if (entries.length === 0) return undefined;
  return Object.fromEntries(entries.map((entry) => [entry.key, entry.value]));
}

function toChapterDisplayTitle(
  chapNum: number,
  title: string | null | undefined,
): string | undefined {
  const trimmedTitle = title?.trim();
  if (trimmedTitle) return trimmedTitle;
  return Number.isFinite(chapNum) ? `Chapter ${chapNum}` : undefined;
}

export class MangashelfExtension implements ExtensionImpl<typeof config> {
  mainRateLimiter = new BasicRateLimiter("main", {
    numberOfRequests: 15,
    bufferInterval: 10,
    ignoreImages: true,
  });

  mainInterceptor = new MainInterceptor("main");
  settingsForm = new SettingsForm({
    onServerUrlChange: async (value) => {
      Application.setState(value, SERVER_URL_STATE_KEY);
      this.settingsForm.setConnectionStatus("Not tested yet.");
    },
    onApiKeyChange: async (value) => {
      Application.setSecureState(value, API_KEY_STATE_KEY);
      this.settingsForm.setConnectionStatus("Not tested yet.");
    },
    onTestConnection: async () => {
      await this.testConnection();
    },
  });
  private cachedSections: ApiDiscoverSection[] = [];

  private getBaseUrl(): string {
    return this.settingsForm.serverUrl.value.replace(/\/+$/, "");
  }

  private getApiKey(): string {
    return this.settingsForm.apiKey.value;
  }

  private apiErrorMessage(status: number, method: string, url: string, responseText: string): string {
    let message = responseText.trim();
    if (message) {
      try {
        const parsed = JSON.parse(message) as { error?: string };
        message = parsed.error || message;
      } catch {
        // Non-JSON responses are still useful when diagnosing proxy or server issues.
      }
    }
    return message
      ? `API request failed: ${status} ${method} ${url}: ${message}`
      : `API request failed: ${status} ${method} ${url}`;
  }

  private async apiRequest<T>(path: string, method = "GET", body?: unknown): Promise<T> {
    const baseUrl = this.getBaseUrl();
    if (!baseUrl) {
      throw new Error("Server URL not configured. Go to Settings to set it up.");
    }

    const url = `${baseUrl}/api${path}`;
    const request: import("@paperback/types").Request = {
      url,
      method,
      headers: {
        Authorization: `Bearer ${this.getApiKey()}`,
        "Content-Type": "application/json",
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    };

    let response: { status: number };
    let data: ArrayBuffer;
    try {
      [response, data] = await Application.scheduleRequest(request);
    } catch (error) {
      const message = error instanceof Error && error.message.trim() ? `: ${error.message}` : "";
      throw new Error(`Network request failed for ${method} ${url}${message}`);
    }
    const jsonString = Application.arrayBufferToUTF8String(data);

    if (response.status < 200 || response.status >= 300) {
      throw new Error(this.apiErrorMessage(response.status, method, url, jsonString));
    }

    try {
      return JSON.parse(jsonString) as T;
    } catch (error) {
      const message = error instanceof Error ? error.message : "invalid JSON";
      throw new Error(`API response was not valid JSON for ${method} ${url}: ${message}`);
    }
  }

  async initialise(): Promise<void> {
    this.loadSettings();
    this.mainRateLimiter.registerInterceptor();
    this.mainInterceptor.registerInterceptor();
  }

  async getSettingsForm(): Promise<Form> {
    this.loadSettings();
    return this.settingsForm;
  }

  private loadSettings(): void {
    this.settingsForm.setPersistedValues({
      serverUrl: (Application.getState(SERVER_URL_STATE_KEY) as string | undefined) ?? "",
      apiKey: (Application.getSecureState(API_KEY_STATE_KEY) as string | undefined) ?? "",
    });
  }

  private async testConnection(): Promise<void> {
    if (!this.getBaseUrl()) {
      this.settingsForm.setConnectionStatus("Enter your server URL first.");
      this.settingsForm.reloadForm();
      return;
    }

    if (!this.getApiKey()) {
      this.settingsForm.setConnectionStatus("Enter your API key first.");
      this.settingsForm.reloadForm();
      return;
    }

    this.settingsForm.setTesting(true);
    this.settingsForm.setConnectionStatus("Testing connection...");
    this.settingsForm.reloadForm();

    try {
      const result = await this.apiRequest<{ user: { username: string } }>("/auth/me");
      this.settingsForm.setConnectionStatus(
        `Connected to the server successfully as ${result.user.username}.`,
      );
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown error";
      this.settingsForm.setConnectionStatus(`Connection failed: ${message}`);
    } finally {
      this.settingsForm.setTesting(false);
      this.settingsForm.reloadForm();
    }
  }

  async getDiscoverSections(): Promise<DiscoverSection[]> {
    try {
      const result = await this.apiRequest<{ sections: ApiDiscoverSection[] }>("/discover");
      this.cachedSections = result.sections;

      const typeMap: Record<ApiDiscoverSection["type"], DiscoverSectionType> = {
        featured: DiscoverSectionType.featured,
        simpleCarousel: DiscoverSectionType.simpleCarousel,
        prominentCarousel: DiscoverSectionType.prominentCarousel,
        chapterUpdates: DiscoverSectionType.chapterUpdates,
        genres: DiscoverSectionType.genres,
      };

      return result.sections.map((section) => ({
        id: section.id,
        title: section.title,
        subtitle: section.subtitle || undefined,
        type: typeMap[section.type],
      }));
    } catch {
      return [];
    }
  }

  async getDiscoverSectionItems(
    section: DiscoverSection,
    metadata: number | undefined,
  ): Promise<PagedResults<DiscoverSectionItem>> {
    void metadata;

    const cachedSection = this.cachedSections.find((entry) => entry.id === section.id);
    if (!cachedSection) return { items: [] };

    return {
      items: cachedSection.items.map((item) => {
        const contentRating = toContentRating(item.contentRating);

        switch (item.type) {
          case "featuredCarouselItem":
            return {
              type: item.type,
              mangaId: item.mangaId,
              imageUrl: item.imageUrl,
              title: item.title,
              supertitle: item.supertitle || undefined,
              metadata: item.metadata ?? undefined,
              contentRating,
            };
          case "simpleCarouselItem":
            return {
              type: item.type,
              mangaId: item.mangaId,
              imageUrl: item.imageUrl,
              title: item.title,
              subtitle: item.subtitle || undefined,
              metadata: item.metadata ?? undefined,
              contentRating,
            };
          case "prominentCarouselItem":
            return {
              type: item.type,
              mangaId: item.mangaId,
              imageUrl: item.imageUrl,
              title: item.title,
              subtitle: item.subtitle || undefined,
              metadata: item.metadata ?? undefined,
              contentRating,
            };
          case "chapterUpdatesCarouselItem":
            return {
              type: item.type,
              mangaId: item.mangaId,
              chapterId: item.chapterId,
              imageUrl: item.imageUrl,
              title: item.title,
              subtitle: toChapterDisplayTitle(item.chapNum, item.subtitle),
              publishDate: toDate(item.publishDate),
              metadata: item.metadata ?? undefined,
              contentRating,
            };
          case "genresCarouselItem":
            return {
              type: item.type,
              name: item.name,
              searchQuery: {
                title: item.searchQuery.title,
                metadata: item.searchQuery.filters,
              } as ExtensionSearchQuery,
              metadata: item.metadata ?? undefined,
              contentRating,
            };
        }
      }) as DiscoverSectionItem[],
    };
  }

  async getSortingOptions(query: SearchQuery<Metadata>): Promise<SortingOption[]> {
    void query;
    return SORTING_OPTIONS;
  }

  private searchFilters(): SearchFilter[] {
    return [
      {
        type: "multiselect",
        id: "contentRating",
        title: "Content Rating",
        options: [
          { id: "SAFE", value: "Safe" },
          { id: "MATURE", value: "Mature" },
          { id: "ADULT", value: "Adult" },
        ],
        value: {},
        allowExclusion: false,
        allowEmptySelection: true,
        maximum: 3,
      },
      {
        type: "multiselect",
        id: "status",
        title: "Status",
        options: [
          { id: "Ongoing", value: "Ongoing" },
          { id: "Completed", value: "Completed" },
          { id: "Hiatus", value: "Hiatus" },
          { id: "Cancelled", value: "Cancelled" },
        ],
        value: {},
        allowExclusion: false,
        allowEmptySelection: true,
        maximum: 4,
      },
      {
        type: "input",
        id: "tags",
        title: "Tags",
        placeholder: "Action, Comedy",
        value: "",
      },
      {
        type: "input",
        id: "minRating",
        title: "Minimum Rating",
        placeholder: "0",
        value: "",
      },
      {
        type: "input",
        id: "maxRating",
        title: "Maximum Rating",
        placeholder: "10",
        value: "",
      },
    ];
  }

  async getAdvancedSearchForm(query: ExtensionSearchQuery): Promise<SearchFilterForm> {
    return new SearchFilterForm(query.metadata, this.searchFilters());
  }

  async getSearchResults(
    query: ExtensionSearchQuery,
    metadata?: Metadata,
    sortingOption?: SortingOption,
  ): Promise<PagedResults<SearchResultItem>> {
    try {
      void metadata;

      const params = new URLSearchParams();
      if (query.title) params.set("q", query.title);
      for (const filter of query.metadata ?? []) {
        switch (filter.id) {
          case "contentRating":
            for (const rating of getIncludedFilterValues(filter.value)) {
              params.append("contentRating", rating);
            }
            break;
          case "status":
            for (const status of getIncludedFilterValues(filter.value)) {
              params.append("status", status);
            }
            break;
          case "tags": {
            const tags = getStringFilterValue(filter.value)
              ?.split(",")
              .map((tag) => tag.trim())
              .filter((tag) => tag.length > 0);
            for (const tag of tags ?? []) params.append("tag", tag);
            break;
          }
          case "minRating": {
            const minRating = getStringFilterValue(filter.value);
            if (minRating) params.set("minRating", minRating);
            break;
          }
          case "maxRating": {
            const maxRating = getStringFilterValue(filter.value);
            if (maxRating) params.set("maxRating", maxRating);
            break;
          }
        }
      }
      if (sortingOption?.id) params.set("sort", sortingOption.id);
      const qs = params.size > 0 ? `?${params.toString()}` : "";
      const result = await this.apiRequest<{ manga: ApiManga[] }>(`/manga${qs}`);

      return {
        items: result.manga.map(
          (manga) =>
            ({
              mangaId: manga.id,
              title: manga.primaryTitle,
              subtitle: manga.author || undefined,
              imageUrl: manga.thumbnailUrl || "",
              metadata: {
                chapterCount: manga.chapterCount,
                status: manga.status,
                rating: manga.rating,
              },
              contentRating: toContentRating(manga.contentRating),
            }) satisfies SearchResultItem,
        ),
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown error";
      throw new Error(`Search failed: ${message}`);
    }
  }

  async getMangaDetails(mangaId: string): Promise<SourceManga> {
    const manga = await this.apiRequest<ApiManga>(`/manga/${mangaId}`);

    return {
      mangaId,
      mangaInfo: {
        thumbnailUrl: manga.thumbnailUrl || "",
        synopsis: manga.synopsis || "No synopsis.",
        primaryTitle: manga.primaryTitle,
        secondaryTitles: manga.secondaryTitles,
        contentRating: toContentRating(manga.contentRating) ?? ContentRating.EVERYONE,
        status: manga.status,
        author: manga.author || undefined,
        artist: manga.artist || undefined,
        bannerUrl: manga.bannerUrl || undefined,
        rating: manga.rating ?? undefined,
        tagGroups: manga.tagGroups,
        artworkUrls: manga.artworkUrls,
        additionalInfo: toAdditionalInfo(manga.additionalInfo),
        shareUrl: manga.shareUrl || undefined,
      },
    };
  }

  async getChapters(sourceManga: SourceManga, sinceDate?: Date): Promise<Chapter[]> {
    void sinceDate;

    const result = await this.apiRequest<{ chapters: ApiChapter[] }>(
      `/manga/${sourceManga.mangaId}/chapters`,
    );

    return result.chapters.map((chapter) => ({
      chapterId: chapter.id,
      sourceManga,
      langCode: chapter.langCode,
      chapNum: chapter.chapNum,
      title: toChapterDisplayTitle(chapter.chapNum, chapter.title),
      version: chapter.version || undefined,
      volume: chapter.volume ?? undefined,
      additionalInfo: toAdditionalInfo(chapter.additionalInfo),
      publishDate: toDate(chapter.publishDate),
      creationDate: toDate(chapter.creationDate),
      sortingIndex: chapter.sortingIndex ?? undefined,
    }));
  }

  async getChapterDetails(chapter: Chapter): Promise<ChapterDetails> {
    const result = await this.apiRequest<ApiChapter>(
      `/manga/${chapter.sourceManga.mangaId}/chapters/${chapter.chapterId}`,
    );

    return {
      id: chapter.chapterId,
      mangaId: chapter.sourceManga.mangaId,
      pages: result.pages || [],
    };
  }

  async getManagedLibraryCollections(): Promise<ManagedCollection[]> {
    const result = await this.apiRequest<{ collections: ApiCollection[] }>("/collections");
    return result.collections.map((collection) => ({
      id: collection.id,
      title: collection.title,
    }));
  }

  async getSourceMangaInManagedCollection(collection: ManagedCollection): Promise<SourceManga[]> {
    const result = await this.apiRequest<{ manga: ApiManga[] }>(
      `/collections/${encodeURIComponent(collection.id)}/manga`,
    );
    return result.manga.map((manga) => apiMangaToSourceManga(manga));
  }

  async commitManagedCollectionChanges(changeset: ManagedCollectionChangeset): Promise<void> {
    await this.apiRequest(
      `/collections/${encodeURIComponent(changeset.collection.id)}/changes`,
      "POST",
      {
        additions: changeset.additions.map((manga) => manga.mangaId),
        deletions: changeset.deletions.map((manga) => manga.mangaId),
      },
    );
  }
}

function apiMangaToSourceManga(manga: ApiManga): SourceManga {
  return {
    mangaId: manga.id,
    mangaInfo: {
      thumbnailUrl: manga.thumbnailUrl || "",
      synopsis: manga.synopsis || "No synopsis.",
      primaryTitle: manga.primaryTitle,
      secondaryTitles: manga.secondaryTitles,
      contentRating: toContentRating(manga.contentRating) ?? ContentRating.EVERYONE,
      status: manga.status,
      author: manga.author || undefined,
      artist: manga.artist || undefined,
      bannerUrl: manga.bannerUrl || undefined,
      rating: manga.rating ?? undefined,
      tagGroups: manga.tagGroups,
      artworkUrls: manga.artworkUrls,
      additionalInfo: toAdditionalInfo(manga.additionalInfo),
      shareUrl: manga.shareUrl || undefined,
    },
  } satisfies SourceManga;
}

export const Mangashelf = new MangashelfExtension();
