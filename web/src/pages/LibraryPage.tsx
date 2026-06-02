import { useState, useEffect, useCallback } from "react";
import { Link, useNavigate } from "react-router";
import * as api from "@/lib/api";
import type { Manga, MangaSortOption } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { PageSpinner } from "@/components/ui/Spinner";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";
import "./LibraryPage.css";

const SORT_OPTIONS: { value: MangaSortOption; label: string }[] = [
  { value: "updated_desc", label: "Recently updated" },
  { value: "updated_asc", label: "Oldest updated" },
  { value: "title_asc", label: "Title A-Z" },
  { value: "title_desc", label: "Title Z-A" },
  { value: "rating_desc", label: "Highest rating" },
  { value: "rating_asc", label: "Lowest rating" },
  { value: "chapters_desc", label: "Most chapters" },
  { value: "chapters_asc", label: "Fewest chapters" },
];

export default function LibraryPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const [manga, setManga] = useState<Manga[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [sort, setSort] = useState<MangaSortOption>("updated_desc");

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const fetchManga = useCallback(async () => {
    try {
      const items = await api.listManga({
        q: debouncedSearch || undefined,
        sort,
      });
      setManga(items);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to load library", "error");
    } finally {
      setLoading(false);
    }
  }, [debouncedSearch, sort, toast]);

  useEffect(() => {
    fetchManga();
  }, [fetchManga]);

  if (loading) return <PageSpinner />;

  return (
    <div className="library-page">
      <div className="page-header">
        <div className="page-header__left">
          <div>
            <h1 className="page-title">Library</h1>
            <p className="page-subtitle">
              {manga.length} title{manga.length !== 1 ? "s" : ""}
            </p>
          </div>
        </div>
        <div className="page-header__actions">
          <div className="library-toolbar">
            <div className="library-search">
              <span className="library-search__icon" aria-hidden="true" />
              <input
                className="library-search__input"
                placeholder="Search manga…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <label className="library-sort">
              <select
                className="library-sort__select"
                value={sort}
                aria-label="Sort library"
                onChange={(e) => setSort(e.target.value as MangaSortOption)}
              >
                {SORT_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <span className="library-sort__chevron" aria-hidden="true" />
            </label>
          </div>
          <button className="library-add" type="button" aria-label="Add manga" onClick={() => navigate("/manga/new")}>
            <span className="library-add__icon" aria-hidden="true" />
          </button>
        </div>
      </div>

      {manga.length === 0 && !debouncedSearch && (
        <EmptyState
          icon="📚"
          title="Your library is empty"
          description="Add your first manga to get started. You can import metadata from AniList."
          action={
            <Button onClick={() => navigate("/manga/new")}>
              + Add Manga
            </Button>
          }
        />
      )}

      {manga.length === 0 && debouncedSearch && (
        <EmptyState
          icon="🔍"
          title="No results"
          description={`No manga found matching "${debouncedSearch}"`}
        />
      )}

      <div className="library-grid stagger-children">
        {manga.map((m) => (
          <Link key={m.id} to={`/manga/${m.id}`} className="manga-card">
            <div className="manga-card__cover">
              {m.thumbnailUrl ? (
                <img
                  src={m.thumbnailUrl}
                  alt={m.primaryTitle}
                  className="manga-card__img"
                  loading="lazy"
                />
              ) : (
                <div className="manga-card__placeholder">
                  <span>📖</span>
                </div>
              )}
            </div>
            <div className="manga-card__info">
              <h3 className="manga-card__title truncate">{m.primaryTitle}</h3>
              <div className="manga-card__meta">
                <span className="manga-card__status">{m.status}</span>
                <span className="manga-card__chapters">
                  {m.chapterCount} ch.
                </span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
