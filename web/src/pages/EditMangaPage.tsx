import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router";
import * as api from "@/lib/api";
import type { MangaPayload, Manga } from "@/lib/types";
import { MangaForm } from "@/components/MangaForm";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { PageSpinner } from "@/components/ui/Spinner";
import { useToast } from "@/components/ui/Toast";

export default function EditMangaPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [manga, setManga] = useState<Manga | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [showDelete, setShowDelete] = useState(false);

  useEffect(() => {
    if (!id) return;
    api.getManga(id)
      .then(setManga)
      .catch(() => {
        toast("Manga not found", "error");
        navigate("/");
      })
      .finally(() => setLoading(false));
  }, [id, navigate, toast]);

  if (loading || !manga) return <PageSpinner />;

  const initialPayload: MangaPayload = {
    primaryTitle: manga.primaryTitle,
    secondaryTitles: manga.secondaryTitles,
    synopsis: manga.synopsis,
    thumbnailUrl: manga.thumbnailUrl,
    bannerUrl: manga.bannerUrl,
    contentRating: manga.contentRating,
    status: manga.status,
    artist: manga.artist,
    author: manga.author,
    rating: manga.rating,
    shareUrl: manga.shareUrl,
    artworkUrls: manga.artworkUrls,
    tagGroups: manga.tagGroups,
    additionalInfo: manga.additionalInfo,
  };

  const handleSubmit = async (payload: MangaPayload) => {
    setSaving(true);
    try {
      await api.updateManga(manga.id, payload);
      toast("Manga updated!", "success");
      navigate(`/manga/${manga.id}`);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to update", "error");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    try {
      await api.deleteManga(manga.id);
      toast("Manga deleted", "success");
      navigate("/");
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to delete", "error");
    }
  };

  return (
    <div>
      <div className="page-header">
        <div className="page-header__left">
          <button className="page-header__back" onClick={() => navigate(`/manga/${manga.id}`)}>←</button>
          <div>
            <h1 className="page-title">Edit Manga</h1>
            <p className="page-subtitle">{manga.primaryTitle}</p>
          </div>
        </div>
        <div className="page-header__actions">
          <Button type="submit" form="manga-edit-form" size="sm" loading={saving}>
            Save Changes
          </Button>
          <Button variant="danger" size="sm" onClick={() => setShowDelete(true)}>
            Delete
          </Button>
        </div>
      </div>
      <MangaForm
        formId="manga-edit-form"
        initial={initialPayload}
        onSubmit={handleSubmit}
      />
      <ConfirmDialog
        open={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={handleDelete}
        title="Delete Manga"
        message={`Are you sure you want to delete "${manga.primaryTitle}"? This will also delete all chapters and sources. This action cannot be undone.`}
        confirmLabel="Delete"
        danger
      />
    </div>
  );
}
