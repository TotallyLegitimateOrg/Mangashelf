import { useState, type FormEvent } from "react";
import type { MangaPayload, TagGroup, Tag } from "@/lib/types";
import { CONTENT_RATINGS, MANGA_STATUSES } from "@/lib/types";
import * as api from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { Select } from "@/components/ui/Select";
import { TagInput } from "@/components/ui/TagInput";
import { KeyValueEditor } from "@/components/ui/KeyValueEditor";
import { UrlListEditor } from "@/components/ui/UrlListEditor";
import { ImagePreview } from "@/components/ui/ImagePreview";
import { useToast } from "@/components/ui/Toast";
import "./MangaForm.css";

interface MangaFormProps {
  initial: MangaPayload;
  onSubmit: (payload: MangaPayload) => Promise<void>;
  formId?: string;
}

export function MangaForm({ initial, onSubmit, formId }: MangaFormProps) {
  const { toast } = useToast();
  const [form, setForm] = useState<MangaPayload>(initial);
  const [anilistId, setAnilistId] = useState("");
  const [importLoading, setImportLoading] = useState(false);

  const update = <K extends keyof MangaPayload>(key: K, value: MangaPayload[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleAnilistImport = async () => {
    const id = parseInt(anilistId, 10);
    if (!id || id <= 0) {
      toast("Enter a valid AniList ID", "error");
      return;
    }
    setImportLoading(true);
    try {
      const payload = await api.fetchAniList(id);
      setForm(payload);
      toast("Imported metadata from AniList!", "success");
    } catch (err) {
      toast(api.errorMessage(err, "Failed to import AniList metadata"), "error");
    } finally {
      setImportLoading(false);
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!form.primaryTitle.trim()) {
      toast("Primary title is required", "error");
      return;
    }
    await onSubmit(form);
  };

  /* ---- Tag group management ---- */
  const addTagGroup = () => {
    update("tagGroups", [
      ...form.tagGroups,
      { id: `tg-${Date.now()}`, title: "", tags: [] },
    ]);
  };

  const updateTagGroup = (index: number, field: keyof TagGroup, value: string | Tag[]) => {
    const groups = form.tagGroups.map((g, i) =>
      i === index ? { ...g, [field]: value } : g
    );
    update("tagGroups", groups);
  };

  const removeTagGroup = (index: number) => {
    update("tagGroups", form.tagGroups.filter((_, i) => i !== index));
  };

  const addTag = (groupIndex: number, title: string) => {
    const group = form.tagGroups[groupIndex];
    if (!group) return;
    const tag: Tag = { id: `tag-${Date.now()}`, title };
    updateTagGroup(groupIndex, "tags", [...group.tags, tag]);
  };

  const removeTag = (groupIndex: number, tagIndex: number) => {
    const group = form.tagGroups[groupIndex];
    if (!group) return;
    updateTagGroup(groupIndex, "tags", group.tags.filter((_, i) => i !== tagIndex));
  };

  return (
    <form id={formId} className="manga-form" onSubmit={handleSubmit}>
      {/* Core fields */}
      <div className="manga-form__section">
        <div className="manga-form__section-header">
          <h3 className="manga-form__section-title font-display">Basic Info</h3>
          <div className="manga-form__anilist">
            <input
              className="manga-form__anilist-input"
              placeholder="AniList ID"
              value={anilistId}
              onChange={(e) => setAnilistId(e.target.value)}
            />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleAnilistImport}
              loading={importLoading}
            >
              Import from AniList
            </Button>
          </div>
        </div>
        <div className="manga-form__grid">
          <div className="manga-form__col-full">
            <Input
              label="Primary Title"
              value={form.primaryTitle}
              onChange={(e) => update("primaryTitle", e.target.value)}
              required
              placeholder="e.g. One Piece"
            />
          </div>
          <TagInput
            label="Secondary Titles"
            values={form.secondaryTitles}
            onChange={(v) => update("secondaryTitles", v)}
            placeholder="Add alternative title…"
          />
          <div className="manga-form__col-full">
            <Textarea
              label="Synopsis"
              value={form.synopsis}
              onChange={(e) => update("synopsis", e.target.value)}
              placeholder="Manga synopsis…"
            />
          </div>
          <Input
            label="Author"
            value={form.author}
            onChange={(e) => update("author", e.target.value)}
            placeholder="Author name"
          />
          <Input
            label="Artist"
            value={form.artist}
            onChange={(e) => update("artist", e.target.value)}
            placeholder="Artist name"
          />
          <Select
            label="Status"
            value={form.status}
            onChange={(e) => update("status", e.target.value)}
            options={MANGA_STATUSES.map((s) => ({ value: s, label: s }))}
          />
          <Select
            label="Content Rating"
            value={form.contentRating}
            onChange={(e) => update("contentRating", e.target.value)}
            options={CONTENT_RATINGS.map((r) => ({ value: r, label: r }))}
          />
          <Input
            label="Rating"
            type="number"
            min={0}
            max={10}
            step={0.1}
            value={form.rating ?? ""}
            onChange={(e) =>
              update("rating", e.target.value ? parseFloat(e.target.value) : null)
            }
            placeholder="0 - 10"
          />
          <Input
            label="Share URL"
            value={form.shareUrl}
            onChange={(e) => update("shareUrl", e.target.value)}
            placeholder="https://…"
          />
        </div>
      </div>

      {/* Images */}
      <div className="manga-form__section">
        <h3 className="manga-form__section-title font-display">Images</h3>
        <div className="manga-form__grid">
          <div>
            <Input
              label="Thumbnail URL"
              value={form.thumbnailUrl}
              onChange={(e) => update("thumbnailUrl", e.target.value)}
              placeholder="https://…"
            />
            <div style={{ marginTop: "var(--space-3)" }}>
              <ImagePreview url={form.thumbnailUrl} height={200} />
            </div>
          </div>
          <div>
            <Input
              label="Banner URL"
              value={form.bannerUrl}
              onChange={(e) => update("bannerUrl", e.target.value)}
              placeholder="https://…"
            />
            <div style={{ marginTop: "var(--space-3)" }}>
              <ImagePreview url={form.bannerUrl} height={120} />
            </div>
          </div>
          <div className="manga-form__col-full">
            <UrlListEditor
              label="Artwork URLs"
              values={form.artworkUrls}
              onChange={(v) => update("artworkUrls", v)}
            />
          </div>
        </div>
      </div>

      {/* Tag Groups */}
      <div className="manga-form__section">
        <div className="manga-form__section-header">
          <h3 className="manga-form__section-title font-display">Tag Groups</h3>
          <Button type="button" variant="ghost" size="sm" onClick={addTagGroup}>
            + Add Group
          </Button>
        </div>
        {form.tagGroups.map((group, gi) => (
          <div key={group.id} className="manga-form__tag-group">
            <div className="manga-form__tag-group-header">
              <input
                className="manga-form__tag-group-title"
                value={group.title}
                onChange={(e) => updateTagGroup(gi, "title", e.target.value)}
                placeholder="Group name (e.g. Genres)"
              />
              <button
                type="button"
                className="manga-form__tag-group-remove"
                onClick={() => removeTagGroup(gi)}
              >
                ✕
              </button>
            </div>
            <div className="manga-form__tags">
              {group.tags.map((tag, ti) => (
                <span key={tag.id} className="manga-form__tag">
                  {tag.title}
                  <button
                    type="button"
                    onClick={() => removeTag(gi, ti)}
                    className="manga-form__tag-remove"
                  >
                    ✕
                  </button>
                </span>
              ))}
              <input
                className="manga-form__tag-input"
                placeholder="Add tag…"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    const val = e.currentTarget.value.trim();
                    if (val) {
                      addTag(gi, val);
                      e.currentTarget.value = "";
                    }
                  }
                }}
              />
            </div>
          </div>
        ))}
      </div>

      {/* Additional Info */}
      <div className="manga-form__section">
        <h3 className="manga-form__section-title font-display">Additional Info</h3>
        <KeyValueEditor
          label=""
          entries={form.additionalInfo}
          onChange={(v) => update("additionalInfo", v)}
        />
      </div>

    </form>
  );
}
