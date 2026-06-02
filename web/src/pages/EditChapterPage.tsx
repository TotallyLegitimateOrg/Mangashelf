import { useState, useEffect, type FormEvent } from "react";
import { useParams, useNavigate } from "react-router";
import * as api from "@/lib/api";
import type { ChapterPayload, ChapterDetail } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { UrlListEditor } from "@/components/ui/UrlListEditor";
import { KeyValueEditor } from "@/components/ui/KeyValueEditor";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { PageSpinner } from "@/components/ui/Spinner";
import { useToast } from "@/components/ui/Toast";
import "./ChapterFormPage.css";

export default function EditChapterPage() {
  const { id: mangaId, chapterId } = useParams<{ id: string; chapterId: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [chapter, setChapter] = useState<ChapterDetail | null>(null);
  const [form, setForm] = useState<ChapterPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  useEffect(() => {
    if (!mangaId || !chapterId) return;
    api.getChapter(mangaId, chapterId)
      .then((ch) => {
        setChapter(ch);
        setForm({
          langCode: ch.langCode,
          chapNum: ch.chapNum,
          title: ch.title,
          version: ch.version,
          volume: ch.volume,
          publishDate: ch.publishDate,
          creationDate: ch.creationDate,
          sortingIndex: ch.sortingIndex,
          additionalInfo: ch.additionalInfo,
          pages: ch.pages,
        });
      })
      .catch((err) => {
        toast(api.errorMessage(err, "Failed to load chapter for editing"), "error");
        navigate(`/manga/${mangaId}`);
      })
      .finally(() => setLoading(false));
  }, [mangaId, chapterId, navigate, toast]);

  if (loading || !form || !chapter) return <PageSpinner />;

  const readOnly = chapter.origin.readOnly;

  const update = <K extends keyof ChapterPayload>(key: K, val: ChapterPayload[K]) => {
    setForm((prev) => prev ? { ...prev, [key]: val } : prev);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!mangaId || !chapterId || readOnly) return;
    setSaving(true);
    try {
      await api.updateChapter(mangaId, chapterId, form);
      toast("Chapter updated!", "success");
      navigate(`/manga/${mangaId}`);
    } catch (err) {
      toast(api.errorMessage(err, "Failed to update chapter"), "error");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!mangaId || !chapterId) return;
    try {
      await api.deleteChapter(mangaId, chapterId);
      toast("Chapter deleted", "success");
      navigate(`/manga/${mangaId}`);
    } catch (err) {
      toast(api.errorMessage(err, "Failed to delete chapter"), "error");
    }
  };

  return (
    <div>
      <div className="page-header">
        <div className="page-header__left">
          <button className="page-header__back" onClick={() => navigate(`/manga/${mangaId}`)}>←</button>
          <div>
            <h1 className="page-title">
              {readOnly ? "View Chapter" : "Edit Chapter"}
            </h1>
            <p className="page-subtitle">
              Ch. {chapter.chapNum}{chapter.title ? ` — ${chapter.title}` : ""}
            </p>
          </div>
        </div>
        <div className="page-header__actions">
          {!readOnly && (
            <Button size="sm" loading={saving} onClick={handleSubmit}>
              Save Changes
            </Button>
          )}
          <Button variant="danger" size="sm" onClick={() => setShowDelete(true)}>
            Delete
          </Button>
        </div>
      </div>

      {readOnly && (
        <div className="chapter-form__readonly-notice">
          This chapter is from a proxy source and cannot be edited.
        </div>
      )}

      <form className="chapter-form" onSubmit={handleSubmit}>
        <div className="chapter-form__fields">
          <Input
            label="Title"
            value={form.title}
            onChange={(e) => update("title", e.target.value)}
            disabled={readOnly}
          />
          <div className="chapter-form__row">
            <Input
              label="Chapter Number"
              type="number"
              step="any"
              value={form.chapNum || ""}
              onChange={(e) => update("chapNum", parseFloat(e.target.value) || 0)}
              disabled={readOnly}
              required
            />
            <Input
              label="Volume"
              type="number"
              step="any"
              value={form.volume ?? ""}
              onChange={(e) =>
                update("volume", e.target.value ? parseFloat(e.target.value) : null)
              }
              disabled={readOnly}
            />
          </div>
          <div className="chapter-form__row">
            <Select
              label="Language"
              value={form.langCode}
              onChange={(e) => update("langCode", e.target.value)}
              disabled={readOnly}
              options={[
                { value: "AR", label: "Arabic" },
                { value: "EN", label: "English" },
              ]}
            />
            <Input
              label="Version"
              value={form.version}
              onChange={(e) => update("version", e.target.value)}
              disabled={readOnly}
            />
          </div>
          <div className="chapter-form__row">
            <Input
              label="Publish Date"
              type="date"
              value={form.publishDate?.split("T")[0] ?? ""}
              onChange={(e) =>
                update("publishDate", e.target.value || null)
              }
              disabled={readOnly}
            />
            <Input
              label="Creation Date"
              type="date"
              value={form.creationDate?.split("T")[0] ?? ""}
              onChange={(e) =>
                update("creationDate", e.target.value || null)
              }
              disabled={readOnly}
            />
          </div>
        </div>

        <UrlListEditor
          label="Page URLs"
          values={form.pages}
          onChange={(v) => update("pages", v)}
        />

        <KeyValueEditor
          label="Additional Info"
          entries={form.additionalInfo}
          onChange={(v) => update("additionalInfo", v)}
        />


      </form>

      <ConfirmDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={handleDelete}
        title="Delete Chapter"
        message={`Delete Chapter ${chapter.chapNum}${chapter.title ? ` — ${chapter.title}` : ""}? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
      />
    </div>
  );
}
