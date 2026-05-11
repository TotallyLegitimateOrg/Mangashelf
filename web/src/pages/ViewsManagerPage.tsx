import { useState, useEffect, useCallback, useMemo } from "react";
import * as api from "@/lib/api";
import type {
  ChapterListItem,
  Collection,
  DiscoverSectionConfig,
  DiscoverSectionPayload,
  DiscoverSectionItem,
  ExtensionMetadata,
  Manga,
} from "@/lib/types";
import { DISCOVER_SECTION_TYPES } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Badge } from "@/components/ui/Badge";
import { Modal } from "@/components/ui/Modal";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { PageSpinner } from "@/components/ui/Spinner";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";
import "./ViewsManagerPage.css";

const SECTION_TYPE_LABELS: Record<string, string> = {
  featured: "Featured",
  simpleCarousel: "Simple Carousel",
  prominentCarousel: "Prominent Carousel",
  chapterUpdates: "Chapter Updates",
  genres: "Genres",
};

const ITEM_TYPE_FOR_SECTION: Record<string, string> = {
  featured: "featuredCarouselItem",
  simpleCarousel: "simpleCarouselItem",
  prominentCarousel: "prominentCarouselItem",
  chapterUpdates: "chapterUpdatesCarouselItem",
  genres: "genresCarouselItem",
};

const SECTION_TYPE_DESCRIPTIONS: Record<string, string> = {
  featured: "Large editorial entries for the top of Discover.",
  simpleCarousel: "Compact rails for quick browsing.",
  prominentCarousel: "Bigger covers for high-priority groups.",
  chapterUpdates: "Chapter-driven rows with optional chapter IDs.",
  genres: "Search shortcuts that open filtered Paperback results.",
};

const LIVE_PRESET_OPTIONS: Record<string, { value: string; label: string }[]> = {
  featured: [
    { value: "title_asc", label: "Title A-Z" },
    { value: "title_desc", label: "Title Z-A" },
    { value: "rating_desc", label: "Top Rated" },
    { value: "updated_desc", label: "Recently Updated" },
    { value: "chapters_desc", label: "Most Chapters" },
  ],
  simpleCarousel: [
    { value: "title_asc", label: "Title A-Z" },
    { value: "title_desc", label: "Title Z-A" },
    { value: "rating_desc", label: "Top Rated" },
    { value: "updated_desc", label: "Recently Updated" },
    { value: "chapters_desc", label: "Most Chapters" },
  ],
  prominentCarousel: [
    { value: "title_asc", label: "Title A-Z" },
    { value: "title_desc", label: "Title Z-A" },
    { value: "rating_desc", label: "Top Rated" },
    { value: "updated_desc", label: "Recently Updated" },
    { value: "chapters_desc", label: "Most Chapters" },
  ],
  chapterUpdates: [{ value: "latest_chapters", label: "Latest Chapters" }],
  genres: [
    { value: "genres_top", label: "Top Genres" },
    { value: "genres_az", label: "Genres A-Z" },
    { value: "genres_za", label: "Genres Z-A" },
  ],
};

interface DiscoverGenreOption {
  id: string;
  label: string;
  queryValue: string;
  mangaCount: number;
}

function emptyPayload(): DiscoverSectionPayload {
  return {
    title: "",
    subtitle: "",
    type: "simpleCarousel",
    mode: "manual",
    liveRule: defaultLiveRule("simpleCarousel"),
    items: [],
  };
}

function itemTypeForSection(sectionType: string): string {
  return ITEM_TYPE_FOR_SECTION[sectionType] ?? "simpleCarouselItem";
}

function newItem(sectionType: string): DiscoverSectionItem {
  return {
    id: `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    type: itemTypeForSection(sectionType),
    mangaId: "",
    imageUrl: "",
    title: "",
    subtitle: "",
    supertitle: "",
    name: "",
    searchQuery: sectionType === "genres" ? { title: "", filters: [] } : null,
  };
}

function defaultLiveRule(sectionType: string) {
  const options = livePresetOptionsFor(sectionType);
  return {
    preset: options[0]?.value ?? "title_asc",
    limit: 10,
  };
}

function livePresetOptionsFor(sectionType: string): { value: string; label: string }[] {
  switch (sectionType) {
    case "featured":
      return LIVE_PRESET_OPTIONS.featured!;
    case "prominentCarousel":
      return LIVE_PRESET_OPTIONS.prominentCarousel!;
    case "chapterUpdates":
      return LIVE_PRESET_OPTIONS.chapterUpdates!;
    case "genres":
      return LIVE_PRESET_OPTIONS.genres!;
    default:
      return LIVE_PRESET_OPTIONS.simpleCarousel!;
  }
}

function presetLabel(preset?: string | null): string {
  if (!preset) return "Live";
  const option = Object.values(LIVE_PRESET_OPTIONS)
    .flat()
    .find((entry) => entry.value === preset);
  return option?.label ?? preset;
}

function selectedGenreQueryValue(item: DiscoverSectionItem): string {
  const tagFilter = item.searchQuery?.filters.find((filter) => filter.id === "tags");
  return typeof tagFilter?.value === "string" ? tagFilter.value : "";
}

function createGenreItem(option: DiscoverGenreOption): DiscoverSectionItem {
  return {
    id: `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    type: itemTypeForSection("genres"),
    name: option.label,
    searchQuery: {
      title: option.label,
      filters: [{ id: "tags", value: option.queryValue }],
    },
  };
}

function createDiscoverItemFromManga(
  sectionType: string,
  manga: Manga,
  chapter: ChapterListItem | null = null,
): DiscoverSectionItem {
  const subtitle =
    manga.author ||
    manga.artist ||
    `${manga.chapterCount} chapter${manga.chapterCount !== 1 ? "s" : ""}`;

  if (sectionType === "chapterUpdates") {
    return {
      id: `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      type: itemTypeForSection("chapterUpdates"),
      mangaId: manga.id,
      chapterId: chapter?.id ?? "",
      chapNum: chapter?.chapNum,
      imageUrl: manga.thumbnailUrl,
      title: manga.primaryTitle,
      subtitle: chapter?.title ?? "",
      publishDate: chapter?.publishDate ?? null,
    };
  }

  if (sectionType === "featured") {
    return {
      id: `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      type: itemTypeForSection("featured"),
      mangaId: manga.id,
      imageUrl: manga.thumbnailUrl,
      title: manga.primaryTitle,
      supertitle: "",
    };
  }

  return {
    id: `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    type: itemTypeForSection(sectionType),
    mangaId: manga.id,
    imageUrl: manga.thumbnailUrl,
    title: manga.primaryTitle,
    subtitle,
  };
}

export default function ViewsManagerPage() {
  const { toast } = useToast();
  const [sections, setSections] = useState<DiscoverSectionConfig[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [library, setLibrary] = useState<Manga[]>([]);
  const [extensionMetadata, setExtensionMetadata] = useState<ExtensionMetadata | null>(null);
  const [selectedCollectionId, setSelectedCollectionId] = useState<string | null>(null);
  const [collectionManga, setCollectionManga] = useState<Manga[]>([]);
  const [loading, setLoading] = useState(true);
  const [collectionLoading, setCollectionLoading] = useState(false);
  const [mode, setMode] = useState<"discover" | "collections">("discover");
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<DiscoverSectionPayload>(emptyPayload());
  const [collectionFormOpen, setCollectionFormOpen] = useState(false);
  const [collectionFormTitle, setCollectionFormTitle] = useState("");
  const [editingCollection, setEditingCollection] = useState<Collection | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DiscoverSectionConfig | null>(null);
  const [collectionDeleteTarget, setCollectionDeleteTarget] = useState<Collection | null>(null);
  const [orderChanged, setOrderChanged] = useState(false);
  const [librarySearch, setLibrarySearch] = useState("");
  const [sectionLibrarySearch, setSectionLibrarySearch] = useState("");
  const [repoOpen, setRepoOpen] = useState(false);

  const selectedCollection =
    collections.find((collection) => collection.id === selectedCollectionId) ?? null;
  const selectedMangaIds = useMemo(
    () => new Set(collectionManga.map((manga) => manga.id)),
    [collectionManga],
  );
  const paperbackRepoUrl =
    typeof window === "undefined" ? "/extensions/paperback" : `${window.location.origin}/extensions/paperback`;
  const paperbackInstallUrl = `paperback://addRepo?displayName=Mangashelf&url=${encodeURI(paperbackRepoUrl)}`;
  const libraryById = useMemo(() => {
    return new Map(library.map((manga) => [manga.id, manga]));
  }, [library]);
  const genreOptions = useMemo(() => {
    const counts = new Map<string, DiscoverGenreOption>();

    for (const manga of library) {
      for (const group of manga.tagGroups) {
        for (const tag of group.tags) {
          const queryValue = `${group.title}:${tag.title}`;
          const label = group.title === "Genres" ? tag.title : `${group.title}: ${tag.title}`;
          const existing = counts.get(queryValue);
          if (existing) {
            existing.mangaCount += 1;
            continue;
          }
          counts.set(queryValue, {
            id: queryValue,
            label,
            queryValue,
            mangaCount: 1,
          });
        }
      }
    }

    return [...counts.values()].sort((a, b) => {
      if (b.mangaCount !== a.mangaCount) return b.mangaCount - a.mangaCount;
      return a.label.localeCompare(b.label);
    });
  }, [library]);
  const primaryExtension = extensionMetadata?.sources[0];
  const extensionDevelopers = primaryExtension?.developers?.map((developer) => developer.name).join(", ");

  const fetchSections = useCallback(async () => {
    const sectionData = await api.listDiscoverSections();
    setSections(Array.isArray(sectionData) ? sectionData : []);
  }, []);

  const fetchCollections = useCallback(async () => {
    const collectionData = await api.listCollections();
    const safeCollections = Array.isArray(collectionData) ? collectionData : [];
    setCollections(safeCollections);
    setSelectedCollectionId((current) => current ?? safeCollections[0]?.id ?? null);
  }, []);

  const loadPageData = useCallback(async () => {
    setLoading(true);
    try {
      const [sectionData, collectionData, libraryData] = await Promise.all([
        api.listDiscoverSections(),
        api.listCollections(),
        api.listManga({ sort: "title_asc" }),
      ]);
      const safeCollections = Array.isArray(collectionData) ? collectionData : [];
      setSections(Array.isArray(sectionData) ? sectionData : []);
      setCollections(safeCollections);
      setLibrary(Array.isArray(libraryData) ? libraryData : []);
      setSelectedCollectionId((current) => current ?? safeCollections[0]?.id ?? null);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to load Paperback settings", "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  const fetchExtensionMetadata = useCallback(async () => {
    try {
      setExtensionMetadata(await api.getExtensionMetadata());
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to load extension metadata", "error");
    }
  }, [toast]);

  const fetchCollectionManga = useCallback(async () => {
    if (!selectedCollectionId) {
      setCollectionManga([]);
      return;
    }
    setCollectionLoading(true);
    try {
      const data = await api.listCollectionManga(selectedCollectionId);
      setCollectionManga(Array.isArray(data) ? data : []);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to load collection manga", "error");
    } finally {
      setCollectionLoading(false);
    }
  }, [selectedCollectionId, toast]);

  useEffect(() => {
    loadPageData();
  }, [loadPageData]);

  useEffect(() => {
    fetchExtensionMetadata();
  }, [fetchExtensionMetadata]);

  useEffect(() => {
    fetchCollectionManga();
  }, [fetchCollectionManga]);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyPayload());
    setSectionLibrarySearch("");
    setShowForm(true);
  };

  const openEdit = (section: DiscoverSectionConfig) => {
    setEditingId(section.id);
    setForm({
      title: section.title,
      subtitle: section.subtitle,
      type: section.type,
      mode: section.mode || "manual",
      liveRule: section.liveRule ?? defaultLiveRule(section.type),
      items: section.mode === "live" ? [] : section.items,
    });
    setSectionLibrarySearch("");
    setShowForm(true);
  };

  const buildDiscoverPayload = useCallback(async (): Promise<DiscoverSectionPayload> => {
    if (form.mode === "live") {
      return {
        ...form,
        liveRule: {
          preset: form.liveRule?.preset || defaultLiveRule(form.type).preset,
          limit: Number.isFinite(form.liveRule?.limit) ? Number(form.liveRule?.limit) : 10,
        },
        items: [],
      };
    }

    if (form.type === "genres") {
      return {
        ...form,
        items: form.items.map((item) => {
          const queryValue = selectedGenreQueryValue(item);
          const option = genreOptions.find((entry) => entry.queryValue === queryValue);
          return option
            ? createGenreItem(option)
            : {
                ...item,
                id: item.id || `item-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                type: itemTypeForSection("genres"),
                name: item.name || item.searchQuery?.title || "Genre",
                searchQuery: {
                  title: item.searchQuery?.title || item.name || "Genre",
                  filters: item.searchQuery?.filters ?? [],
                },
              };
        }),
      };
    }

    const items = await Promise.all(
      form.items.map(async (item) => {
        if (!item.mangaId) return item;
        const manga = libraryById.get(item.mangaId);
        if (!manga) return item;

        if (form.type === "chapterUpdates") {
          const chapters = await api.listChapters(manga.id);
          const latestChapter = chapters[chapters.length - 1] ?? null;
          if (!latestChapter) {
            throw new Error(`"${manga.primaryTitle}" needs at least one chapter before it can appear in Chapter Updates.`);
          }
          const nextItem = createDiscoverItemFromManga(form.type, manga, latestChapter);
          return {
            ...nextItem,
            id: item.id || nextItem.id,
          };
        }

        const nextItem = createDiscoverItemFromManga(form.type, manga);
        return {
          ...nextItem,
          id: item.id || nextItem.id,
          supertitle: form.type === "featured" ? item.supertitle ?? "" : nextItem.supertitle,
        };
      }),
    );

    return {
      ...form,
      items,
    };
  }, [form, genreOptions, libraryById]);

  const handleSave = async () => {
    if (!form.title.trim()) {
      toast("Section title is required", "error");
      return;
    }
    if (form.mode === "live") {
      if (!form.liveRule?.preset) {
        toast("Choose a live preset for this section", "error");
        return;
      }
      if (!form.liveRule?.limit || form.liveRule.limit <= 0) {
        toast("Live sections need a limit greater than zero", "error");
        return;
      }
    } else if (form.items.length === 0) {
      toast("Select at least one item for this section", "error");
      return;
    }
    setSaving(true);
    try {
      const payload = await buildDiscoverPayload();
      if (editingId) {
        await api.updateDiscoverSection(editingId, payload);
        toast("Section updated", "success");
      } else {
        await api.createDiscoverSection(payload);
        toast("Section created", "success");
      }
      setShowForm(false);
      fetchSections();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to save", "error");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteDiscoverSection(deleteTarget.id);
      toast("Section deleted", "success");
      setDeleteTarget(null);
      fetchSections();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to delete", "error");
    }
  };

  const moveSection = (index: number, direction: -1 | 1) => {
    const newSections = [...sections];
    const target = index + direction;
    if (target < 0 || target >= newSections.length) return;
    [newSections[index]!, newSections[target]!] = [newSections[target]!, newSections[index]!];
    setSections(newSections);
    setOrderChanged(true);
  };

  const saveOrder = async () => {
    try {
      await api.reorderDiscoverSections(sections.map((s) => s.id));
      toast("Order saved", "success");
      setOrderChanged(false);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to save order", "error");
    }
  };

  const openCreateCollection = () => {
    setEditingCollection(null);
    setCollectionFormTitle("");
    setCollectionFormOpen(true);
  };

  const openRenameCollection = (collection: Collection) => {
    setEditingCollection(collection);
    setCollectionFormTitle(collection.title);
    setCollectionFormOpen(true);
  };

  const saveCollection = async () => {
    if (!collectionFormTitle.trim()) {
      toast("Collection title is required", "error");
      return;
    }
    try {
      const saved = editingCollection
        ? await api.updateCollection(editingCollection.id, { title: collectionFormTitle })
        : await api.createCollection({ title: collectionFormTitle });
      setCollectionFormOpen(false);
      setSelectedCollectionId(saved.id);
      await fetchCollections();
      toast(editingCollection ? "Collection renamed" : "Collection created", "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to save collection", "error");
    }
  };

  const deleteCollection = async () => {
    if (!collectionDeleteTarget) return;
    try {
      await api.deleteCollection(collectionDeleteTarget.id);
      setCollectionDeleteTarget(null);
      setSelectedCollectionId(null);
      await fetchCollections();
      toast("Collection deleted", "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to delete collection", "error");
    }
  };

  const toggleCollectionManga = async (manga: Manga) => {
    if (!selectedCollectionId) return;
    const included = selectedMangaIds.has(manga.id);
    try {
      await api.applyCollectionChanges(selectedCollectionId, {
        additions: included ? [] : [manga.id],
        deletions: included ? [manga.id] : [],
      });
      await Promise.all([fetchCollectionManga(), fetchCollections()]);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to update collection", "error");
    }
  };

  const moveCollection = async (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= collections.length) return;
    const next = [...collections];
    [next[index]!, next[target]!] = [next[target]!, next[index]!];
    setCollections(next);
    try {
      await api.reorderCollections(next.map((collection) => collection.id));
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to reorder collections", "error");
      fetchCollections();
    }
  };

  const removeItem = (index: number) => {
    setForm((prev) => ({
      ...prev,
      items: prev.items.filter((_, i) => i !== index),
    }));
  };

  const moveItem = (index: number, direction: -1 | 1) => {
    setForm((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.items.length) return prev;
      const items = [...prev.items];
      [items[index]!, items[target]!] = [items[target]!, items[index]!];
      return { ...prev, items };
    });
  };

  const updateItemSupertitle = (index: number, supertitle: string) => {
    setForm((prev) => ({
      ...prev,
      items: prev.items.map((item, itemIndex) =>
        itemIndex === index ? { ...item, supertitle } : item,
      ),
    }));
  };

  if (loading) return <PageSpinner />;

  const isGenres = form.type === "genres";
  const isLive = form.mode === "live";
  const isFeatured = form.type === "featured";
  const isChapterUpdates = form.type === "chapterUpdates";
  const livePresetOptions = livePresetOptionsFor(form.type);
  const filteredLibrary = library.filter((manga) =>
    manga.primaryTitle.toLowerCase().includes(librarySearch.toLowerCase()),
  );
  const sectionFilteredLibrary = library.filter((manga) =>
    manga.primaryTitle.toLowerCase().includes(sectionLibrarySearch.toLowerCase()),
  );
  const filteredGenreOptions = genreOptions.filter((option) =>
    option.label.toLowerCase().includes(sectionLibrarySearch.toLowerCase()),
  );
  const selectedSectionMangaIds = new Set(form.items.map((item) => item.mangaId).filter(Boolean));
  const selectedGenreQueryValues = new Set(
    form.items.map((item) => selectedGenreQueryValue(item)).filter(Boolean),
  );
  const sectionPreviewItems = (section: DiscoverSectionConfig) => (section.items ?? []).slice(0, 5);

  const coverForItem = (item: DiscoverSectionItem) =>
    item.imageUrl || (item.mangaId ? libraryById.get(item.mangaId)?.thumbnailUrl : "") || "";

  const titleForItem = (item: DiscoverSectionItem) =>
    item.title || item.name || (item.mangaId ? libraryById.get(item.mangaId)?.primaryTitle : "") || "Untitled";

  const liveSummary = (section: DiscoverSectionConfig) =>
    section.mode === "live"
      ? `${presetLabel(section.liveRule?.preset)} · ${section.liveRule?.limit ?? 10} items`
      : "";

  const toggleMangaItem = (manga: Manga) => {
    setForm((prev) => {
      const alreadyAdded = prev.items.some((item) => item.mangaId === manga.id);
      if (alreadyAdded) {
        return {
          ...prev,
          items: prev.items.filter((item) => item.mangaId !== manga.id),
        };
      }

      const item = newItem(prev.type);
      return {
        ...prev,
        items: [
          ...prev.items,
          { ...createDiscoverItemFromManga(prev.type, manga), id: item.id },
        ],
      };
    });
  };

  const toggleGenreItem = (option: DiscoverGenreOption) => {
    setForm((prev) => {
      const alreadyAdded = prev.items.some((item) => selectedGenreQueryValue(item) === option.queryValue);
      if (alreadyAdded) {
        return {
          ...prev,
          items: prev.items.filter((item) => selectedGenreQueryValue(item) !== option.queryValue),
        };
      }

      return {
        ...prev,
        items: [...prev.items, createGenreItem(option)],
      };
    });
  };

  const copyRepositoryUrl = () => {
    navigator.clipboard.writeText(paperbackRepoUrl);
    toast("Repository URL copied", "success");
  };

  const installRepository = () => {
    window.location.href = paperbackInstallUrl;
  };

  return (
    <div className="paperback-page">
      <div className="page-header">
        <div className="page-header__left">
          <div>
            <h2 className="page-title">Manage Paperback</h2>
          </div>
        </div>
        <div className="page-header__actions">
          <div className="paperback-switch" aria-label="Paperback management area">
            <button
              className={mode === "discover" ? "paperback-switch__item paperback-switch__item--active" : "paperback-switch__item"}
              onClick={() => setMode("discover")}
            >
              Discover <span>{sections.length}</span>
            </button>
            <button
              className={mode === "collections" ? "paperback-switch__item paperback-switch__item--active" : "paperback-switch__item"}
              onClick={() => setMode("collections")}
            >
              Collections <span>{collections.length}</span>
            </button>
          </div>
          <button
            type="button"
            className={`paperback-repo-toggle ${repoOpen ? "paperback-repo-toggle--open" : ""}`}
            onClick={() => setRepoOpen((open) => !open)}
            aria-expanded={repoOpen}
          >
            Repository
            {primaryExtension?.version && <span>v{primaryExtension.version}</span>}
          </button>
          {mode === "discover" && orderChanged && (
            <Button size="sm" onClick={saveOrder}>Save order</Button>
          )}
          {mode === "discover" ? (
            <Button onClick={openCreate}>Add section</Button>
          ) : (
            <Button onClick={openCreateCollection}>Add collection</Button>
          )}
        </div>
      </div>

      {repoOpen && (
        <section className="paperback-repo-panel" aria-label="Paperback repository">
          <div className="paperback-repo-panel__identity">
            <h3 className="paperback-repo-panel__title font-display">
              {primaryExtension?.name ?? "Mangashelf"}
            </h3>
            <p>
              {primaryExtension?.version ? `Version ${primaryExtension.version}` : "Extension repository"}
              {extensionDevelopers ? ` by ${extensionDevelopers}` : ""}
              {extensionMetadata?.buildTime ? ` · Built ${new Date(extensionMetadata.buildTime).toLocaleString()}` : ""}
            </p>
          </div>
          <code className="paperback-repo-panel__url">{paperbackRepoUrl}</code>
          <div className="paperback-repo-panel__actions">
            <Button size="sm" variant="secondary" onClick={copyRepositoryUrl}>Copy URL</Button>
            <Button size="sm" onClick={installRepository}>Install</Button>
          </div>
        </section>
      )}

      {mode === "discover" && sections.length === 0 && (
        <EmptyState
          icon="D"
          title="No discover sections"
          description="Create sections to populate the Paperback discover tab."
          action={<Button onClick={openCreate}>Add section</Button>}
        />
      )}

      {mode === "discover" && <div className="views-list stagger-children">
        {sections.map((section, i) => (
          <div key={section.id} className="views-card">
            <div className="views-card__header">
              <span className="views-card__order">{String(i + 1).padStart(2, "0")}</span>
              <div className="views-card__title-row">
                <h3 className="views-card__title font-display">{section.title}</h3>
                <Badge variant="accent">{SECTION_TYPE_LABELS[section.type] ?? section.type}</Badge>
                {section.mode === "live" && <Badge variant="info">Live</Badge>}
              </div>
              {section.subtitle && (
                <p className="views-card__subtitle">{section.subtitle}</p>
              )}
            </div>
            <p className="views-card__description">
              {section.mode === "live"
                ? liveSummary(section)
                : SECTION_TYPE_DESCRIPTIONS[section.type] ?? "Custom Paperback Discover section."}
            </p>
            <div className="views-card__preview" aria-label={`${section.title} preview`}>
              {sectionPreviewItems(section).map((item, itemIndex) => {
                const cover = coverForItem(item);
                return (
                  <span key={item.id || itemIndex} className="views-card__cover" title={titleForItem(item)}>
                    {cover ? <img src={cover} alt={`${titleForItem(item)} cover`} /> : <span>{titleForItem(item).slice(0, 1)}</span>}
                  </span>
                );
              })}
              {(section.items ?? []).length === 0 && <span className="views-card__empty-preview">No items yet</span>}
            </div>
            <div className="views-card__meta">
              <span>{(section.items ?? []).length} item{(section.items ?? []).length !== 1 ? "s" : ""}</span>
              <span>Appears #{i + 1}</span>
            </div>
            <div className="views-card__actions">
              <button className="views-card__move" onClick={() => moveSection(i, -1)} disabled={i === 0} aria-label="Move section up">↑</button>
              <button className="views-card__move" onClick={() => moveSection(i, 1)} disabled={i === sections.length - 1} aria-label="Move section down">↓</button>
              <Button size="sm" variant="ghost" onClick={() => openEdit(section)}>Edit</Button>
              <Button size="sm" variant="danger" onClick={() => setDeleteTarget(section)}>Delete</Button>
            </div>
          </div>
        ))}
      </div>}

      {mode === "collections" && (
        collections.length === 0 ? (
          <EmptyState
            icon="C"
            title="No collections"
            description="Create a collection to group manga for Paperback."
            action={<Button onClick={openCreateCollection}>Add collection</Button>}
          />
        ) : (
          <div className="collections-shell">
            <aside className="collections-list">
              {collections.map((collection, index) => (
                <button
                  key={collection.id}
                  className={`collection-row ${collection.id === selectedCollectionId ? "collection-row--active" : ""}`}
                  onClick={() => setSelectedCollectionId(collection.id)}
                >
                  <span className="collection-row__main">
                    <span className="collection-row__title">{collection.title}</span>
                    <span className="collection-row__count">{collection.mangaCount} manga</span>
                  </span>
                  <span className="collection-row__order">
                    <span onClick={(event) => { event.stopPropagation(); moveCollection(index, -1); }} aria-label="Move collection up">↑</span>
                    <span onClick={(event) => { event.stopPropagation(); moveCollection(index, 1); }} aria-label="Move collection down">↓</span>
                  </span>
                </button>
              ))}
            </aside>

            <section className="collections-detail">
              {selectedCollection && (
                <>
                  <div className="collections-detail__header">
                    <div>
                      <h2>{selectedCollection.title}</h2>
                      <p>{collectionManga.length} title{collectionManga.length !== 1 ? "s" : ""}</p>
                    </div>
                    <div className="collections-detail__actions">
                      <Button size="sm" variant="ghost" onClick={() => openRenameCollection(selectedCollection)}>Rename</Button>
                      <Button size="sm" variant="danger" onClick={() => setCollectionDeleteTarget(selectedCollection)}>Delete</Button>
                    </div>
                  </div>

                  <Input
                    label="Find manga to add or remove"
                    value={librarySearch}
                    onChange={(event) => setLibrarySearch(event.target.value)}
                    placeholder="Search library"
                  />

                  {collectionLoading ? (
                    <PageSpinner />
                  ) : (
                    <div className="collections-manga-grid">
                      {filteredLibrary.map((manga) => {
                        const active = selectedMangaIds.has(manga.id);
                        return (
                          <button
                            key={manga.id}
                            className={`collections-manga ${active ? "collections-manga--active" : ""}`}
                            onClick={() => toggleCollectionManga(manga)}
                          >
                            {manga.thumbnailUrl ? <img src={manga.thumbnailUrl} alt={`${manga.primaryTitle} cover`} /> : <span />}
                            <strong>{manga.primaryTitle}</strong>
                            <small>{active ? "Included" : "Add"}</small>
                          </button>
                        );
                      })}
                      {filteredLibrary.length === 0 && (
                        <p className="collections-empty-result">No manga match that search.</p>
                      )}
                    </div>
                  )}
                </>
              )}
            </section>
          </div>
        )
      )}

      {/* Create / Edit Modal */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={editingId ? "Edit Section" : "New Section"}
        actions={
          <Button size="sm" onClick={handleSave} loading={saving} disabled={form.mode === "manual" && form.items.length === 0}>
            {editingId ? "Save Changes" : "Create Section"}
          </Button>
        }
        width="680px"
      >
        <div className="views-form">
          <div className="views-form__grid">
            <Input
              label="Title"
              value={form.title}
              onChange={(e) => setForm((p) => ({ ...p, title: e.target.value }))}
              required
            />
            <Input
              label="Subtitle"
              value={form.subtitle}
              onChange={(e) => setForm((p) => ({ ...p, subtitle: e.target.value }))}
            />
            <Select
              label="Section Type"
              value={form.type}
              onChange={(e) => {
                const newType = e.target.value;
                const switchingSelectionMode =
                  form.mode === "manual" && (
                    (form.type === "genres" && newType !== "genres") ||
                    (form.type !== "genres" && newType === "genres")
                  );
                const nextLiveRule = form.liveRule && LIVE_PRESET_OPTIONS[newType]?.some((option) => option.value === form.liveRule?.preset)
                  ? form.liveRule
                  : defaultLiveRule(newType);
                setForm((p) => ({
                  ...p,
                  type: newType,
                  liveRule: nextLiveRule,
                  items: switchingSelectionMode
                    ? []
                    : p.items.map((item) => ({
                        ...item,
                        type: itemTypeForSection(newType),
                      })),
                }));
                if (switchingSelectionMode && form.items.length > 0) {
                  toast("Switched picker mode and cleared the previous selections.", "info");
                }
              }}
              options={DISCOVER_SECTION_TYPES.map((t) => ({
                value: t,
                label: SECTION_TYPE_LABELS[t] ?? t,
              }))}
            />
            <Select
              label="Section Mode"
              value={form.mode}
              onChange={(e) => {
                const nextMode = e.target.value;
                setForm((prev) => ({
                  ...prev,
                  mode: nextMode,
                  liveRule: prev.liveRule ?? defaultLiveRule(prev.type),
                }));
              }}
              options={[
                { value: "manual", label: "Manual" },
                { value: "live", label: "Live" },
              ]}
            />
          </div>

          {isLive && (
            <>
              <div className="views-form__grid views-form__grid--live">
                <Select
                  label="Live Preset"
                  value={form.liveRule?.preset ?? livePresetOptions[0]?.value ?? ""}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      liveRule: {
                        preset: event.target.value,
                        limit: prev.liveRule?.limit ?? 10,
                      },
                    }))
                  }
                  options={livePresetOptions}
                />
                <Input
                  label="Item Limit"
                  type="number"
                  min={1}
                  value={String(form.liveRule?.limit ?? 10)}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      liveRule: {
                        preset: prev.liveRule?.preset ?? defaultLiveRule(prev.type).preset,
                        limit: Number.parseInt(event.target.value, 10) || 0,
                      },
                    }))
                  }
                />
              </div>
              <p className="views-form__hint">
                Live sections are generated each time Paperback requests Discover.
              </p>
            </>
          )}

          {!isLive && (
            <>
              <div className="views-form__items-header">
                <span className="input-group__label">Selected items ({form.items.length})</span>
              </div>
              <p className="views-form__hint">
                Choose the titles or genres you want to show here. Mangashelf will fill the Paperback-specific fields automatically.
              </p>
            </>
          )}

          {!isLive && !isGenres && (
            <div className="views-form__library">
              <Input
                label="Choose titles"
                value={sectionLibrarySearch}
                onChange={(event) => setSectionLibrarySearch(event.target.value)}
                placeholder="Search manga"
              />
              <div className="views-form__library-results">
                {sectionFilteredLibrary.slice(0, 8).map((manga) => {
                  const alreadyAdded = selectedSectionMangaIds.has(manga.id);
                  return (
                    <button
                      key={manga.id}
                      type="button"
                      className={`views-form__library-item ${alreadyAdded ? "views-form__library-item--added" : ""}`}
                      onClick={() => toggleMangaItem(manga)}
                    >
                      {manga.thumbnailUrl ? <img src={manga.thumbnailUrl} alt={`${manga.primaryTitle} cover`} /> : <span />}
                      <strong>{manga.primaryTitle}</strong>
                      <small>{alreadyAdded ? "Remove" : "Add"}</small>
                    </button>
                  );
                })}
                {sectionFilteredLibrary.length === 0 && (
                  <p className="views-form__empty-picker">No manga match that search.</p>
                )}
              </div>
            </div>
          )}

          {!isLive && isGenres && (
            <div className="views-form__library">
              <Input
                label="Choose genres"
                value={sectionLibrarySearch}
                onChange={(event) => setSectionLibrarySearch(event.target.value)}
                placeholder="Search tags or groups"
              />
              <div className="views-form__genre-results">
                {filteredGenreOptions.slice(0, 24).map((option) => {
                  const active = selectedGenreQueryValues.has(option.queryValue);
                  return (
                    <button
                      key={option.id}
                      type="button"
                      className={`views-form__genre-item ${active ? "views-form__genre-item--active" : ""}`}
                      onClick={() => toggleGenreItem(option)}
                    >
                      <strong>{option.label}</strong>
                      <small>
                        {active ? "Selected" : "Select"} · {option.mangaCount} match{option.mangaCount !== 1 ? "es" : ""}
                      </small>
                    </button>
                  );
                })}
                {filteredGenreOptions.length === 0 && (
                  <p className="views-form__empty-picker">No genres match that search.</p>
                )}
              </div>
            </div>
          )}

          {!isLive && <div className="views-form__selected-list">
            {form.items.length === 0 ? (
              <div className="views-form__selected-empty">
                {isGenres ? "Select genres to populate this section." : "Select manga from your library to populate this section."}
              </div>
            ) : (
              form.items.map((item, i) => {
                const cover = coverForItem(item);
                const queryValue = selectedGenreQueryValue(item);
                const genreOption = genreOptions.find((option) => option.queryValue === queryValue);
                return (
                  <div key={item.id || i} className="views-form__selected-item">
                    <span className="views-form__item-num">#{i + 1}</span>
                    {!isGenres && (
                      cover ? (
                        <img src={cover} alt={`${titleForItem(item)} cover`} className="views-form__selected-cover" />
                      ) : (
                        <span className="views-form__selected-cover views-form__selected-cover--empty">
                          {titleForItem(item).slice(0, 1)}
                        </span>
                      )
                    )}
                    <div className="views-form__selected-copy">
                      <strong>{isGenres ? genreOption?.label ?? item.name ?? item.searchQuery?.title ?? "Genre" : titleForItem(item)}</strong>
                      <small>
                        {isGenres
                          ? `${genreOption?.mangaCount ?? 0} matching title${genreOption?.mangaCount === 1 ? "" : "s"}`
                          : isFeatured
                            ? item.supertitle || "Optional eyebrow text shown above the featured title."
                          : isChapterUpdates
                            ? "Latest chapter is resolved automatically when you save."
                            : item.subtitle || "Included in this section"}
                      </small>
                      {isFeatured && (
                        <label className="views-form__inline-field">
                          <span>Supertitle</span>
                          <input
                            type="text"
                            value={item.supertitle ?? ""}
                            onChange={(event) => updateItemSupertitle(i, event.target.value)}
                            placeholder="New & Noteworthy"
                          />
                        </label>
                      )}
                    </div>
                    <div className="views-form__selected-actions">
                      <button
                        type="button"
                        className="views-card__move"
                        onClick={() => moveItem(i, -1)}
                        disabled={i === 0}
                        aria-label="Move item up"
                      >
                        ↑
                      </button>
                      <button
                        type="button"
                        className="views-card__move"
                        onClick={() => moveItem(i, 1)}
                        disabled={i === form.items.length - 1}
                        aria-label="Move item down"
                      >
                        ↓
                      </button>
                      <button
                        type="button"
                        className="views-form__item-remove"
                        onClick={() => removeItem(i)}
                      >
                        Remove
                      </button>
                    </div>
                  </div>
                );
              })
            )}
          </div>}

        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Section"
        message={deleteTarget ? `Delete "${deleteTarget.title}" and all its items?` : ""}
        confirmLabel="Delete"
        danger
      />

      <Modal
        open={collectionFormOpen}
        onClose={() => setCollectionFormOpen(false)}
        title={editingCollection ? "Rename Collection" : "Create Collection"}
        actions={
          <>
            <Button variant="ghost" onClick={() => setCollectionFormOpen(false)}>Cancel</Button>
            <Button onClick={saveCollection}>{editingCollection ? "Save" : "Create"}</Button>
          </>
        }
      >
        <Input
          label="Title"
          value={collectionFormTitle}
          onChange={(event) => setCollectionFormTitle(event.target.value)}
          autoFocus
        />
      </Modal>

      <ConfirmDialog
        open={!!collectionDeleteTarget}
        title="Delete collection?"
        message={`Delete "${collectionDeleteTarget?.title ?? ""}"? Manga in the collection will remain in the library.`}
        confirmLabel="Delete"
        danger
        onConfirm={deleteCollection}
        onClose={() => setCollectionDeleteTarget(null)}
      />
    </div>
  );
}
