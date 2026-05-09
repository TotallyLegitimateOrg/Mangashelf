import { useState } from "react";
import { useNavigate } from "react-router";
import * as api from "@/lib/api";
import { emptyMangaPayload } from "@/lib/types";
import type { MangaPayload } from "@/lib/types";
import { MangaForm } from "@/components/MangaForm";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";

export default function CreateMangaPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (payload: MangaPayload) => {
    setSaving(true);
    try {
      const manga = await api.createManga(payload);
      toast("Manga created!", "success");
      navigate(`/manga/${manga.id}`);
    } catch (err) {
      toast(err instanceof Error ? err.message : "Failed to create manga", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div className="page-header__left">
          <button className="page-header__back" onClick={() => navigate(-1)}>←</button>
          <h1 className="page-title">Add Manga</h1>
        </div>
        <div className="page-header__actions">
          <Button type="submit" form="manga-create-form" size="sm" loading={saving}>
            Create Manga
          </Button>
        </div>
      </div>
      <MangaForm
        formId="manga-create-form"
        initial={emptyMangaPayload()}
        onSubmit={handleSubmit}
      />
    </div>
  );
}
