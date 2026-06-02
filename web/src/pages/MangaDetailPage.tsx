import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useParams, useNavigate } from "react-router";
import * as api from "@/lib/api";
import type { Manga, ChapterListItem, ChapterSource, ChapterSourceSyncLog } from "@/lib/types";
import { PROVIDERS } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { PageSpinner } from "@/components/ui/Spinner";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";
import "./MangaDetailPage.css";

/** Look up a human-readable config summary for a source via its provider definition. */
function sourceConfigSummary(src: ChapterSource): string {
  const def = PROVIDERS.find((p) => p.id === src.provider);
  if (def) return def.configSummary(src.config);
  return JSON.stringify(src.config);
}

type SyncLogViewEntry = ChapterSourceSyncLog & {
  message: string;
  timeLabel: string;
  dateLabel: string;
};

const MAX_COLLAPSED_SYNC_LOGS_PER_SOURCE = 3;

const BULK_LANGUAGE_OPTIONS = [
  { value: "", label: "Keep current language" },
  { value: "AR", label: "Arabic" },
  { value: "EN", label: "English" },
];

function formatSyncLogTimestamp(createdAt: string): { timeLabel: string; dateLabel: string } {
  const date = new Date(createdAt);
  return {
    timeLabel: date.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    }),
    dateLabel: date.toLocaleDateString([], {
      month: "short",
      day: "numeric",
    }),
  };
}

function relativeSyncAge(createdAt: string): string {
  const deltaMinutes = Math.max(0, Math.round((Date.now() - new Date(createdAt).getTime()) / 60000));
  if (deltaMinutes < 1) return "Just now";
  if (deltaMinutes < 60) return `${deltaMinutes}m ago`;

  const deltaHours = Math.round(deltaMinutes / 60);
  if (deltaHours < 24) return `${deltaHours}h ago`;

  const deltaDays = Math.round(deltaHours / 24);
  return `${deltaDays}d ago`;
}

function relativeFutureAge(targetAt: string): string {
  const deltaMinutes = Math.max(0, Math.round((new Date(targetAt).getTime() - Date.now()) / 60000));
  if (deltaMinutes < 1) return "now";
  if (deltaMinutes < 60) return `in ${deltaMinutes}m`;

  const deltaHours = Math.round(deltaMinutes / 60);
  if (deltaHours < 24) return `in ${deltaHours}h`;

  const deltaDays = Math.round(deltaHours / 24);
  return `in ${deltaDays}d`;
}

function nextSyncAt(lastSyncedAt: string | null, syncIntervalMinutes: number): string | null {
  if (!lastSyncedAt || syncIntervalMinutes <= 0) return null;
  const lastSynced = new Date(lastSyncedAt).getTime();
  if (Number.isNaN(lastSynced)) return null;
  return new Date(lastSynced + syncIntervalMinutes * 60_000).toISOString();
}

function formatScheduledSync(lastSyncedAt: string | null, syncIntervalMinutes: number): string {
  const nextAt = nextSyncAt(lastSyncedAt, syncIntervalMinutes);
  if (!nextAt) return `Runs every ${syncIntervalMinutes}m`;
  return `Next sync ${relativeFutureAge(nextAt)}`;
}

function syncResultMessage(result: {
  insertedCount: number;
  updatedCount: number;
  unchangedCount: number;
  skippedCount: number;
}): string {
  const parts = [
    `${result.insertedCount} inserted`,
    `${result.updatedCount} updated`,
    `${result.unchangedCount} unchanged`,
  ];
  if (result.skippedCount > 0) {
    parts.push(`${result.skippedCount} skipped because matching chapters already exist`);
  }
  return parts.join(", ");
}

function syncLogMessage(entry: ChapterSourceSyncLog): string {
  if (entry.status === "error") return entry.error || "Sync failed";
  return syncResultMessage(entry);
}

function syncLogStatusVariant(status: ChapterSourceSyncLog["status"]): "success" | "error" {
  return status === "error" ? "error" : "success";
}

function groupSyncLogsBySource(entries: ChapterSourceSyncLog[]): Map<string, SyncLogViewEntry[]> {
  const grouped = new Map<string, SyncLogViewEntry[]>();

  for (const entry of entries) {
    const sourceEntries = grouped.get(entry.sourceId) ?? [];
    const timestamp = formatSyncLogTimestamp(entry.createdAt);
    sourceEntries.push({
      ...entry,
      message: syncLogMessage(entry),
      timeLabel: timestamp.timeLabel,
      dateLabel: timestamp.dateLabel,
    });
    grouped.set(entry.sourceId, sourceEntries);
  }

  return grouped;
}

export default function MangaDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [manga, setManga] = useState<Manga | null>(null);
  const [chapters, setChapters] = useState<ChapterListItem[]>([]);
  const [sources, setSources] = useState<ChapterSource[]>([]);
  const [syncLogs, setSyncLogs] = useState<ChapterSourceSyncLog[]>([]);
  const [expandedSourceLogs, setExpandedSourceLogs] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"chapters" | "sources" | "info">("chapters");

  const [syncingSourceId, setSyncingSourceId] = useState<string | null>(null);
  const [clearingLogsSourceId, setClearingLogsSourceId] = useState<string | null>(null);

  const syncLogsBySource = useMemo(() => groupSyncLogsBySource(syncLogs), [syncLogs]);

  /* Chapter delete */
  const [deleteChapter, setDeleteChapter] = useState<ChapterListItem | null>(null);

  /* Bulk chapter metadata */
  const [selectedChapterIds, setSelectedChapterIds] = useState<Set<string>>(new Set());
  const [bulkMetadataOpen, setBulkMetadataOpen] = useState(false);
  const [bulkLangCode, setBulkLangCode] = useState("");
  const [bulkVersion, setBulkVersion] = useState("");
  const [savingBulkMetadata, setSavingBulkMetadata] = useState(false);
  const [bulkMetadataError, setBulkMetadataError] = useState<string | null>(null);

  /* Source unlink */
  const [unlinkSourceId, setUnlinkSourceId] = useState<string | null>(null);

  /* Sorting / reorder state */
  const [reorderedChapters, setReorderedChapters] = useState<ChapterListItem[] | null>(null);
  const [savingOrder, setSavingOrder] = useState(false);

  /* Drag state */
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);
  const dragNodeRef = useRef<HTMLDivElement | null>(null);

  const fetchAll = useCallback(async () => {
    if (!id) return;
    try {
      const [m, c, sourceData] = await Promise.all([
        api.getManga(id),
        api.listChapters(id),
        api.listChapterSources(id),
      ]);
      setManga(m);
      setChapters(c);
      setSources(sourceData.sources);
      setSyncLogs(sourceData.syncLogs);
    } catch {
      toast("Failed to load manga", "error");
      navigate("/");
    } finally {
      setLoading(false);
    }
  }, [id, navigate, toast]);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  useEffect(() => {
    setSelectedChapterIds((current) => {
      if (current.size === 0) return current;
      const editableIds = new Set(chapters.filter((ch) => !ch.origin.readOnly).map((ch) => ch.id));
      const next = new Set(Array.from(current).filter((chapterId) => editableIds.has(chapterId)));
      return next.size === current.size ? current : next;
    });
  }, [chapters]);

  const toggleSourceLogs = useCallback((sourceId: string) => {
    setExpandedSourceLogs((current) => {
      const next = new Set(current);
      if (next.has(sourceId)) {
        next.delete(sourceId);
      } else {
        next.add(sourceId);
      }
      return next;
    });
  }, []);

  /* Derive displayed chapters */
  const displayedChapters = reorderedChapters ?? chapters;
  const hasPendingReorder = reorderedChapters !== null;
  const editableChapters = useMemo(() => chapters.filter((ch) => !ch.origin.readOnly), [chapters]);
  const selectedCount = selectedChapterIds.size;
  const allEditableSelected = editableChapters.length > 0
    && editableChapters.every((ch) => selectedChapterIds.has(ch.id));

  /* Detect if the saved server order differs from chapter-number order */
  const hasCustomSavedOrder = chapters.some((ch) => ch.sortingIndex != null);

  const clearChapterSelection = useCallback(() => {
    setSelectedChapterIds(new Set());
  }, []);

  const toggleChapterSelection = useCallback((chapterId: string, checked: boolean) => {
    setSelectedChapterIds((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(chapterId);
      } else {
        next.delete(chapterId);
      }
      return next;
    });
  }, []);

  const selectAllEditableChapters = useCallback(() => {
    setSelectedChapterIds(new Set(editableChapters.map((ch) => ch.id)));
  }, [editableChapters]);

  const openBulkMetadataEditor = useCallback(() => {
    setBulkLangCode("");
    setBulkVersion("");
    setBulkMetadataError(null);
    setBulkMetadataOpen(true);
  }, []);

  const closeBulkMetadataEditor = useCallback(() => {
    if (savingBulkMetadata) return;
    setBulkMetadataOpen(false);
    setBulkMetadataError(null);
  }, [savingBulkMetadata]);

  /* Drag-and-drop handlers */
  const handleDragStart = (index: number, e: React.DragEvent) => {
    setDragIndex(index);
    e.dataTransfer.effectAllowed = "move";
    // Use a transparent image so we get the native drag ghost
    const el = e.currentTarget as HTMLElement;
    dragNodeRef.current = el as HTMLDivElement;
    e.dataTransfer.setDragImage(el, 20, 20);
  };

  const handleDragOver = (index: number, e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    if (dragIndex !== null && index !== dragIndex) {
      setDragOverIndex(index);
    }
  };

  const handleDragEnd = () => {
    if (dragIndex !== null && dragOverIndex !== null && dragIndex !== dragOverIndex) {
      const list = [...(reorderedChapters ?? chapters)];
      const [moved] = list.splice(dragIndex, 1);
      list.splice(dragOverIndex, 0, moved!);
      setReorderedChapters(list);
    }
    setDragIndex(null);
    setDragOverIndex(null);
    dragNodeRef.current = null;
  };

  const handleDragLeave = () => {
    setDragOverIndex(null);
  };

  /* Move chapter up/down with buttons */
  const moveChapter = (fromIndex: number, direction: "up" | "down") => {
    const toIndex = direction === "up" ? fromIndex - 1 : fromIndex + 1;
    const list = [...(reorderedChapters ?? chapters)];
    if (toIndex < 0 || toIndex >= list.length) return;
    [list[fromIndex], list[toIndex]] = [list[toIndex]!, list[fromIndex]!];
    setReorderedChapters(list);
  };

  /* Save custom order */
  const handleSaveOrder = async () => {
    if (!id || !reorderedChapters) return;
    setSavingOrder(true);
    try {
      const order = reorderedChapters
        .filter((ch) => !ch.origin.readOnly)
        .map((ch) => ch.id);
      await api.reorderChapters(id, order);
      toast("Chapter order saved", "success");
      setReorderedChapters(null);
      fetchAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to save order", "error");
    } finally {
      setSavingOrder(false);
    }
  };

  /* Reset to chapter number order */
  const handleResetToChapterOrder = async () => {
    if (!id) return;
    setSavingOrder(true);
    try {
      await api.reorderChapters(id, []);
      toast("Chapters sorted by number", "success");
      setReorderedChapters(null);
      fetchAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to reset order", "error");
    } finally {
      setSavingOrder(false);
    }
  };

  const handleCancelReorder = () => {
    setReorderedChapters(null);
  };

  const handleSyncNow = async (sourceId: string) => {
    if (!id) return;
    setSyncingSourceId(sourceId);
    try {
      const result = await api.triggerSync(id, sourceId);
      const message = syncResultMessage(result);
      toast(message, "success");
      fetchAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Sync failed";
      toast(message, "error");
      fetchAll();
    } finally {
      setSyncingSourceId(null);
    }
  };

  const handleDeleteChapter = async () => {
    if (!id || !deleteChapter) return;
    try {
      await api.deleteChapter(id, deleteChapter.id);
      toast("Chapter deleted", "success");
      setDeleteChapter(null);
      fetchAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to delete chapter", "error");
    }
  };

  const handleBulkMetadataSubmit = async () => {
    if (!id || selectedChapterIds.size === 0) return;
    const langCode = bulkLangCode.trim();
    const version = bulkVersion.trim();
    if (!langCode && !version) {
      setBulkMetadataError("Enter a language, a version, or both.");
      return;
    }
    setSavingBulkMetadata(true);
    setBulkMetadataError(null);
    try {
      const result = await api.bulkUpdateChapterMetadata(id, {
        chapterIds: Array.from(selectedChapterIds),
        ...(langCode ? { langCode } : {}),
        ...(version ? { version } : {}),
      });
      setChapters(result.chapters);
      setReorderedChapters(null);
      clearChapterSelection();
      setBulkMetadataOpen(false);
      toast(`Updated ${result.updatedCount} chapter${result.updatedCount !== 1 ? "s" : ""}`, "success");
      fetchAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to update chapters";
      setBulkMetadataError(message);
      toast(message, "error");
    } finally {
      setSavingBulkMetadata(false);
    }
  };

  const handleUnlinkSource = async () => {
    if (!id || !unlinkSourceId) return;
    try {
      await api.unlinkChapterSource(id, unlinkSourceId);
      toast("Source unlinked", "success");
      setUnlinkSourceId(null);
      fetchAll();
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to unlink source", "error");
    }
  };

  const handleClearSourceLogs = async (sourceId: string) => {
    if (!id) return;
    setClearingLogsSourceId(sourceId);
    try {
      await api.clearChapterSourceLogs(id, sourceId);
      setSyncLogs((current) => current.filter((entry) => entry.sourceId !== sourceId));
      setExpandedSourceLogs((current) => {
        const next = new Set(current);
        next.delete(sourceId);
        return next;
      });
      toast("Sync activity cleared", "success");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to clear sync activity", "error");
    } finally {
      setClearingLogsSourceId(null);
    }
  };

  const handleChapterClick = (ch: ChapterListItem) => {
    if (ch.origin.readOnly) {
      // Read-only proxy chapters — don't navigate to the edit page
      return;
    }
    navigate(`/manga/${manga!.id}/chapters/${ch.id}/edit`);
  };

  const handleChapterKeyDown = (event: React.KeyboardEvent, ch: ChapterListItem) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      handleChapterClick(ch);
    }
  };

  /** Build a compact origin badge label for imported/source-linked chapters */
  const originBadge = (ch: ChapterListItem) => {
    if (ch.origin.readOnly) return "Proxy";
    if (ch.origin.mode === "sync" || ch.origin.mode === "import_once") {
      const providerLabel = ch.origin.provider
        ? PROVIDERS.find((p) => p.id === ch.origin.provider)?.label ?? ch.origin.provider
        : null;
      if (providerLabel) return `${providerLabel} · ${ch.origin.mode === "import_once" ? "Imported" : "Synced"}`;
    }
    return null;
  };

  if (loading || !manga) return <PageSpinner />;

  return (
    <div className="manga-detail">
      {/* Hero */}
      <div className="manga-hero">
        {manga.bannerUrl && (
          <img src={manga.bannerUrl} alt="" className="manga-hero__banner" />
        )}
        <div className="manga-hero__gradient" />
        <div className="manga-hero__content">
          <div className="manga-hero__cover">
            {manga.thumbnailUrl ? (
              <img src={manga.thumbnailUrl} alt={manga.primaryTitle} />
            ) : (
              <span className="manga-hero__cover-placeholder">📖</span>
            )}
          </div>
          <div className="manga-hero__info">
            <h1 className="manga-hero__title font-display">{manga.primaryTitle}</h1>
            {(manga.author || manga.artist) && (
              <p className="manga-hero__credits">
                {manga.author}{manga.author && manga.artist && " · "}{manga.artist}
              </p>
            )}
            <div className="manga-hero__badges">
              <Badge>{manga.status}</Badge>
              <Badge>{manga.contentRating}</Badge>
              {manga.rating !== null && (
                <span className="manga-hero__rating">★ {manga.rating.toFixed(1)}</span>
              )}
              <span className="manga-hero__chapter-count">
                {manga.chapterCount} chapter{manga.chapterCount !== 1 ? "s" : ""}
              </span>
            </div>
            {manga.synopsis && (
              <p className="manga-hero__synopsis line-clamp-3">{manga.synopsis}</p>
            )}
            <div className="manga-hero__actions">
              <Button size="sm" onClick={() => navigate(`/manga/${manga.id}/edit`)}>
                Edit
              </Button>
              <Button size="sm" variant="secondary" onClick={() => navigate(`/manga/${manga.id}/chapters/new`)}>
                Add Chapter
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="manga-tabs">
        <button
          className={`manga-tabs__tab ${activeTab === "chapters" ? "manga-tabs__tab--active" : ""}`}
          onClick={() => setActiveTab("chapters")}
        >
          Chapters ({chapters.length})
        </button>
        <button
          className={`manga-tabs__tab ${activeTab === "sources" ? "manga-tabs__tab--active" : ""}`}
          onClick={() => setActiveTab("sources")}
        >
          Sources ({sources.length})
        </button>
        <button
          className={`manga-tabs__tab ${activeTab === "info" ? "manga-tabs__tab--active" : ""}`}
          onClick={() => setActiveTab("info")}
        >
          Info
        </button>
      </div>

      {/* Chapters tab */}
      {activeTab === "chapters" && (
        <div className="manga-section">
          {chapters.length === 0 ? (
            <EmptyState
              icon="📄"
              title="No chapters yet"
              description="Add chapters manually, upload a CBZ/ZIP, or import from a remote source."
              action={
                <Button onClick={() => navigate(`/manga/${manga.id}/chapters/new`)}>
                  Add Chapter
                </Button>
              }
            />
          ) : (
            <>
              {/* Sort controls */}
              {(hasPendingReorder || hasCustomSavedOrder) && (
                <div className="chapter-sort-bar">
                  <div className="chapter-sort-bar__actions">
                    {hasCustomSavedOrder && !hasPendingReorder && (
                      <Button size="sm" variant="secondary" onClick={handleResetToChapterOrder} loading={savingOrder}>
                        Reset to Ch. #
                      </Button>
                    )}
                    {hasPendingReorder && (
                      <>
                        <Button size="sm" variant="secondary" onClick={handleCancelReorder}>
                          Cancel
                        </Button>
                        <Button size="sm" onClick={handleSaveOrder} loading={savingOrder}>
                          Save Order
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              )}

              <div className={`chapter-bulk-bar ${selectedCount > 0 ? "chapter-bulk-bar--active" : ""}`}>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={selectAllEditableChapters}
                  disabled={editableChapters.length === 0 || allEditableSelected}
                >
                  Select all editable
                </Button>
                {selectedCount > 0 && (
                  <div className="chapter-bulk-bar__actions">
                    <span className="chapter-bulk-bar__count">
                      {selectedCount} selected
                    </span>
                    <Button size="sm" variant="secondary" onClick={clearChapterSelection}>
                      Clear
                    </Button>
                    <Button size="sm" onClick={openBulkMetadataEditor}>
                      Edit language/version
                    </Button>
                  </div>
                )}
              </div>

              {/* Chapter list */}
              <div className={`chapter-list ${hasPendingReorder ? "chapter-list--reordering" : ""}`}>
                {displayedChapters.map((ch, index) => {
                  const isReadOnly = ch.origin.readOnly;
                  const badge = originBadge(ch);
                  const isDragging = dragIndex === index;
                  const isDragOver = dragOverIndex === index;
                  const canDrag = !isReadOnly;
                  const isSelected = selectedChapterIds.has(ch.id);

                  return (
                    <div
                      key={ch.id}
                      className={[
                        "chapter-row",
                        isReadOnly ? "chapter-row--readonly" : "",
                        isDragging ? "chapter-row--dragging" : "",
                        isDragOver ? "chapter-row--drag-over" : "",
                      ].filter(Boolean).join(" ")}
                      role="button"
                      tabIndex={0}
                      draggable={canDrag}
                      onDragStart={(e) => handleDragStart(index, e)}
                      onDragOver={(e) => handleDragOver(index, e)}
                      onDragEnd={handleDragEnd}
                      onDragLeave={handleDragLeave}
                      onClick={() => !isDragging && handleChapterClick(ch)}
                      onKeyDown={(event) => handleChapterKeyDown(event, ch)}
                      aria-pressed={isSelected}
                    >
                      <button
                        type="button"
                        className="chapter-row__select"
                        disabled={isReadOnly}
                        aria-label={`${isSelected ? "Deselect" : "Select"} chapter ${ch.chapNum}`}
                        aria-pressed={isSelected}
                        onClick={(event) => {
                          event.stopPropagation();
                          toggleChapterSelection(ch.id, !isSelected);
                        }}
                        onKeyDown={(event) => event.stopPropagation()}
                      >
                        <span className="chapter-row__select-mark">✓</span>
                      </button>

                      {/* Drag handle */}
                      {canDrag && (
                        <div className="chapter-row__drag-handle" title="Drag to reorder">
                          <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
                            <circle cx="4" cy="2" r="1.2" />
                            <circle cx="8" cy="2" r="1.2" />
                            <circle cx="4" cy="6" r="1.2" />
                            <circle cx="8" cy="6" r="1.2" />
                            <circle cx="4" cy="10" r="1.2" />
                            <circle cx="8" cy="10" r="1.2" />
                          </svg>
                        </div>
                      )}

                      <div className="chapter-row__main">
                        <span className="chapter-row__num">Ch. {ch.chapNum}</span>
                        <span className="chapter-row__title truncate">
                          {ch.title || "Untitled"}
                        </span>
                        {ch.version && (
                          <Badge variant="default">{ch.version}</Badge>
                        )}
                        {badge && (
                          <Badge variant="info">{badge}</Badge>
                        )}
                      </div>
                      <div className="chapter-row__meta">
                        <span>{ch.langCode}</span>
                        <span>{ch.pageCount} pg</span>
                        {ch.publishDate && (
                          <span>{new Date(ch.publishDate).toLocaleDateString()}</span>
                        )}
                      </div>

                      {/* Reorder buttons (custom mode) */}
                      {canDrag && (
                        <div className="chapter-row__move-btns">
                          <button
                            className="chapter-row__move-btn"
                            onClick={(e) => { e.stopPropagation(); moveChapter(index, "up"); }}
                            disabled={index === 0}
                            title="Move up"
                          >
                            ↑
                          </button>
                          <button
                            className="chapter-row__move-btn"
                            onClick={(e) => { e.stopPropagation(); moveChapter(index, "down"); }}
                            disabled={index === displayedChapters.length - 1}
                            title="Move down"
                          >
                            ↓
                          </button>
                        </div>
                      )}

                      {!isReadOnly && (
                        <div className="chapter-row__actions">
                          <button
                            className="chapter-row__action chapter-row__action--danger"
                            onClick={(event) => {
                              event.stopPropagation();
                              setDeleteChapter(ch);
                            }}
                          >
                            🗑
                          </button>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>
      )}

      {/* Sources tab — management only, no Add Source action */}
      {activeTab === "sources" && (
        <div className="manga-section">
          {sources.length === 0 ? (
            <EmptyState
              icon="🔗"
              title="No sources linked"
              description="Use the Add Chapter page to link a remote source in proxy or sync mode."
            />
          ) : (
            <div className="source-list">
              {sources.map((src) => {
                const sourceLogs = syncLogsBySource.get(src.id) ?? [];
                const isExpanded = expandedSourceLogs.has(src.id);
                const visibleLogs = isExpanded
                  ? sourceLogs
                  : sourceLogs.slice(0, MAX_COLLAPSED_SYNC_LOGS_PER_SOURCE);
                const hiddenLogCount = Math.max(
                  0,
                  sourceLogs.length - MAX_COLLAPSED_SYNC_LOGS_PER_SOURCE
                );
                const latestLog = sourceLogs[0] ?? null;
                const recentErrorCount = sourceLogs.filter((entry) => entry.status === "error").length;
                const scheduledSyncLabel = formatScheduledSync(
                  src.lastSyncedAt,
                  src.syncIntervalMinutes
                );
                return (
                  <div key={src.id} className="source-card">
                    <div className="source-card__header">
                      <Badge variant="accent">{src.provider}</Badge>
                      <Badge>{src.mode}</Badge>
                    </div>
                    <p className="source-card__config truncate">{sourceConfigSummary(src)}</p>
                    {src.lastError && (
                      <p className="source-card__error">{src.lastError}</p>
                    )}
                    {src.mode === "sync" && sourceLogs.length > 0 && (
                      <div className="source-card__sync-log" aria-label="Recent sync activity">
                        <div className="source-card__sync-log-summary">
                          <div className="source-card__sync-log-summary-copy">
                            <span className="source-card__sync-log-kicker">Recent sync activity</span>
                            <div className="source-card__sync-log-headline">
                              <Badge variant={syncLogStatusVariant(latestLog?.status ?? "success")}>
                                {latestLog?.status === "error" ? "Needs attention" : "Healthy"}
                              </Badge>
                              {latestLog?.status === "error" && (
                                <span className="source-card__sync-log-summary-text">
                                  {latestLog.error || "Latest sync failed"}
                                </span>
                              )}
                            </div>
                          </div>
                          <div className="source-card__sync-log-summary-meta">
                            {latestLog && (
                              <span className="source-card__sync-log-summary-age">
                                {relativeSyncAge(latestLog.createdAt)}
                              </span>
                            )}
                            <span className="source-card__sync-log-summary-stats">
                              {scheduledSyncLabel}
                              {" · "}
                              {`${sourceLogs.length} runs`}
                              {recentErrorCount > 0 ? ` · ${recentErrorCount} errors` : ""}
                            </span>
                          </div>
                        </div>
                        {visibleLogs.map((entry) => (
                          <div
                            key={entry.id}
                            className={`source-card__sync-log-row source-card__sync-log-row--${entry.status}`}
                          >
                            <span className="source-card__sync-log-dot" />
                            <div className="source-card__sync-log-body">
                              <span className="source-card__sync-log-message">{entry.message}</span>
                              <time className="source-card__sync-log-time" dateTime={entry.createdAt}>
                                {entry.dateLabel} · {entry.timeLabel}
                              </time>
                            </div>
                          </div>
                        ))}
                        {hiddenLogCount > 0 && (
                          <button
                            type="button"
                            className="source-card__sync-log-toggle"
                            onClick={() => toggleSourceLogs(src.id)}
                            aria-expanded={isExpanded}
                          >
                            {isExpanded
                              ? "Show less"
                              : `Show ${hiddenLogCount} more ${hiddenLogCount === 1 ? "entry" : "entries"}`}
                          </button>
                        )}
                      </div>
                    )}
                    <div className="source-card__footer">
                      <div className="source-card__actions">
                        {src.mode === "sync" && (
                          <>
                            <Button
                              size="sm"
                              variant="secondary"
                              onClick={() => handleSyncNow(src.id)}
                              loading={syncingSourceId === src.id}
                            >
                              Sync Now
                            </Button>
                            {sourceLogs.length > 0 && (
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() => handleClearSourceLogs(src.id)}
                                loading={clearingLogsSourceId === src.id}
                              >
                                Clear Logs
                              </Button>
                            )}
                          </>
                        )}
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => setUnlinkSourceId(src.id)}
                        >
                          Unlink
                        </Button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Info tab */}
      {activeTab === "info" && (
        <div className="manga-section manga-info">
          {manga.secondaryTitles.length > 0 && (
            <div className="manga-info__block">
              <h4 className="manga-info__label">Alternative Titles</h4>
              <ul className="manga-info__alt-titles">
                {manga.secondaryTitles.map((t, i) => (
                  <li key={i}>{t}</li>
                ))}
              </ul>
            </div>
          )}

          {manga.synopsis && (
            <div className="manga-info__block">
              <h4 className="manga-info__label">Synopsis</h4>
              <p className="manga-info__synopsis">{manga.synopsis}</p>
            </div>
          )}

          {manga.tagGroups.map((group) => (
            <div key={group.id} className="manga-info__block">
              <h4 className="manga-info__label">{group.title}</h4>
              <div className="manga-info__tags">
                {group.tags.map((tag) => (
                  <Badge key={tag.id} variant="accent">{tag.title}</Badge>
                ))}
              </div>
            </div>
          ))}

          {manga.additionalInfo.length > 0 && (
            <div className="manga-info__block">
              <h4 className="manga-info__label">Additional Information</h4>
              <table className="manga-info__table">
                <tbody>
                  {manga.additionalInfo.map((info, i) => (
                    <tr key={i}>
                      <td className="manga-info__key">{info.key}</td>
                      <td>{info.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {manga.artworkUrls.length > 0 && (
            <div className="manga-info__block">
              <h4 className="manga-info__label">Artwork</h4>
              <div className="manga-info__gallery">
                {manga.artworkUrls.map((url, i) => (
                  <img key={i} src={url} alt={`Artwork ${i + 1}`} className="manga-info__art" loading="lazy" />
                ))}
              </div>
            </div>
          )}

          {manga.shareUrl && (
            <div className="manga-info__block">
              <h4 className="manga-info__label">Share URL</h4>
              <a href={manga.shareUrl} target="_blank" rel="noopener noreferrer" className="manga-info__link">
                {manga.shareUrl}
              </a>
            </div>
          )}
        </div>
      )}

      <Modal
        open={bulkMetadataOpen}
        onClose={closeBulkMetadataEditor}
        title="Edit selected chapters"
        width="420px"
        actions={
          <>
            <Button size="sm" variant="secondary" onClick={closeBulkMetadataEditor} disabled={savingBulkMetadata}>
              Cancel
            </Button>
            <Button size="sm" onClick={handleBulkMetadataSubmit} loading={savingBulkMetadata}>
              Apply
            </Button>
          </>
        }
      >
        <div className="chapter-bulk-modal">
          <p className="chapter-bulk-modal__summary">
            Update {selectedCount} selected chapter{selectedCount !== 1 ? "s" : ""}. Leave a field blank to keep its current value.
          </p>
          <Select
            label="Language"
            value={bulkLangCode}
            onChange={(event) => setBulkLangCode(event.target.value)}
            options={BULK_LANGUAGE_OPTIONS}
          />
          <Input
            label="Version"
            value={bulkVersion}
            onChange={(event) => setBulkVersion(event.target.value)}
            placeholder="Default"
          />
          {bulkMetadataError && (
            <p className="chapter-bulk-modal__error">{bulkMetadataError}</p>
          )}
        </div>
      </Modal>

      {/* Confirm delete chapter */}
      <ConfirmDialog
        open={!!deleteChapter}
        onClose={() => setDeleteChapter(null)}
        onConfirm={handleDeleteChapter}
        title="Delete Chapter"
        message={deleteChapter ? `Delete Chapter ${deleteChapter.chapNum}${deleteChapter.title ? ` — ${deleteChapter.title}` : ""}?` : ""}
        confirmLabel="Delete"
        danger
      />

      {/* Confirm unlink source */}
      <ConfirmDialog
        open={!!unlinkSourceId}
        onClose={() => setUnlinkSourceId(null)}
        onConfirm={handleUnlinkSource}
        title="Unlink Source"
        message="This will remove the linked source and all its proxy chapters. Continue?"
        confirmLabel="Unlink"
        danger
      />
    </div>
  );
}
