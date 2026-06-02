import { useState, useEffect, useCallback, useRef } from "react";
import * as api from "@/lib/api";
import type { APIKey, BuildInfo } from "@/lib/types";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { PageSpinner } from "@/components/ui/Spinner";
import { useToast } from "@/components/ui/Toast";
import "./SettingsPage.css";

export default function SettingsPage() {
  const { user } = useAuth();
  const { toast } = useToast();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [buildInfo, setBuildInfo] = useState<BuildInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const restoreInputRef = useRef<HTMLInputElement | null>(null);

  /* Create key */
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [creating, setCreating] = useState(false);
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  /* Delete key */
  const [deleteTarget, setDeleteTarget] = useState<APIKey | null>(null);

  /* Backup and restore */
  const [downloadingBackup, setDownloadingBackup] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<File | null>(null);
  const [restoringBackup, setRestoringBackup] = useState(false);

  const fetchKeys = useCallback(async () => {
    try {
      const [keysResult, buildInfoResult] = await Promise.allSettled([
        api.listAPIKeys(),
        api.getBuildInfo(),
      ]);

      if (keysResult.status === "fulfilled") {
        setKeys(keysResult.value);
      } else {
        throw keysResult.reason;
      }

      if (buildInfoResult.status === "fulfilled") {
        setBuildInfo(buildInfoResult.value);
      } else {
        toast(
          api.errorMessage(buildInfoResult.reason, "Failed to load app version"),
          "error"
        );
      }
    } catch (err) {
      toast(api.errorMessage(err, "Failed to load settings"), "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    fetchKeys();
  }, [fetchKeys]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const result = await api.createAPIKey(newKeyName || "default");
      setCreatedKey(result.apiKey);
      setNewKeyName("");
      fetchKeys();
    } catch (err) {
      toast(api.errorMessage(err, "Failed to create API key"), "error");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteAPIKey(deleteTarget.id);
      toast("API key deleted", "success");
      setDeleteTarget(null);
      fetchKeys();
    } catch (err) {
      toast(api.errorMessage(err, "Failed to delete API key"), "error");
    }
  };

  const copyKey = () => {
    if (createdKey) {
      navigator.clipboard.writeText(createdKey);
      toast("Copied to clipboard!", "success");
    }
  };

  const handleDownloadBackup = async () => {
    setDownloadingBackup(true);
    try {
      await api.downloadBackup();
      toast("Backup downloaded", "success");
    } catch (err) {
      toast(api.errorMessage(err, "Failed to download backup"), "error");
    } finally {
      setDownloadingBackup(false);
    }
  };

  const handleRestoreBackup = async () => {
    if (!restoreTarget) return;
    setRestoringBackup(true);
    try {
      const result = await api.restoreBackup(restoreTarget);
      toast(
        `Restored ${result.mangaCount} manga, ${result.chapterCount} chapters, ${result.collectionCount} collections, ${result.discoverSectionCount} discover sections, and ${result.chapterSourceCount} sources`,
        "success"
      );
      setRestoreTarget(null);
      if (restoreInputRef.current) {
        restoreInputRef.current.value = "";
      }
    } catch (err) {
      toast(api.errorMessage(err, "Failed to restore backup"), "error");
    } finally {
      setRestoringBackup(false);
    }
  };

  const handleSelectRestoreFile = () => {
    if (restoreInputRef.current) {
      restoreInputRef.current.value = "";
    }
    restoreInputRef.current?.click();
  };

  const handleRestoreFileSelected = (file: File | null) => {
    setRestoreTarget(file);
  };

  if (loading) return <PageSpinner />;

  return (
    <div className="settings-page">
      <div className="page-header">
        <h1 className="page-title">Settings</h1>
      </div>

      <div className="settings-section">
        <h2 className="settings-section__title font-display">App</h2>
        <div className="settings-card">
          <div className="settings-card__row">
            <span className="settings-card__label">Version</span>
            <span className="settings-card__value">{buildInfo?.version ?? "dev"}</span>
          </div>
          <div className="settings-card__row">
            <span className="settings-card__label">Commit</span>
            <span className="settings-card__value settings-card__value--mono">
              {buildInfo?.commit ?? "unknown"}
            </span>
          </div>
          <div className="settings-card__row">
            <span className="settings-card__label">Built At</span>
            <span className="settings-card__value">
              {buildInfo?.builtAt ? new Date(buildInfo.builtAt).toLocaleString() : "Unknown"}
            </span>
          </div>
        </div>
      </div>

      {/* Account */}
      <div className="settings-section">
        <h2 className="settings-section__title font-display">Account</h2>
        <div className="settings-card">
          <div className="settings-card__row">
            <span className="settings-card__label">Username</span>
            <span className="settings-card__value">{user?.username}</span>
          </div>
          <div className="settings-card__row">
            <span className="settings-card__label">User ID</span>
            <span className="settings-card__value settings-card__value--mono">{user?.id}</span>
          </div>
        </div>
      </div>

      {/* API Keys */}
      <div className="settings-section">
        <div className="settings-section__header">
          <h2 className="settings-section__title font-display">API Keys</h2>
          <Button size="sm" onClick={() => { setShowCreate(true); setCreatedKey(null); }}>
            + Create Key
          </Button>
        </div>

        {keys.length === 0 ? (
          <div className="settings-card">
            <p className="settings-card__empty">No API keys created yet.</p>
          </div>
        ) : (
          <div className="settings-keys">
            {keys.map((key) => (
              <div key={key.id} className="settings-key">
                <div className="settings-key__info">
                  <span className="settings-key__name">{key.name}</span>
                  <span className="settings-key__prefix">{key.prefix}…</span>
                </div>
                <div className="settings-key__meta">
                  <span className="settings-key__date">
                    Created {new Date(key.createdAt * 1000).toLocaleDateString()}
                  </span>
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => setDeleteTarget(key)}
                  >
                    Delete
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Backup and restore */}
      <div className="settings-section">
        <h2 className="settings-section__title font-display">Backup & Restore</h2>
        <div className="settings-card settings-backup-card">
          <div className="settings-backup-row">
            <div className="settings-backup-row__main">
              <span className="settings-backup-row__icon" aria-hidden="true">↓</span>
              <div className="settings-backup-row__copy">
                <span className="settings-backup-row__title">Export Library</span>
                <span className="settings-backup-row__detail">JSON backup</span>
              </div>
            </div>
            <Button size="sm" onClick={handleDownloadBackup} loading={downloadingBackup}>
              Download
            </Button>
          </div>
          <div className="settings-backup-row settings-backup-row--restore">
            <div className="settings-backup-row__main">
              <span className="settings-backup-row__icon settings-backup-row__icon--danger" aria-hidden="true">↺</span>
              <div className="settings-backup-row__copy">
                <span className="settings-backup-row__title">Restore Library</span>
                <span className="settings-backup-row__detail">Replace current data</span>
              </div>
            </div>
            <Button
              size="sm"
              type="button"
              variant="danger"
              loading={restoringBackup}
              onClick={handleSelectRestoreFile}
            >
              Restore
            </Button>
          </div>
          <input
            ref={restoreInputRef}
            className="settings-restore__input"
            type="file"
            accept="application/json,.json"
            disabled={restoringBackup}
            onChange={(event) => handleRestoreFileSelected(event.target.files?.[0] ?? null)}
          />
        </div>
      </div>

      {/* Create key modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title={createdKey ? "API Key Created" : "Create API Key"}
        width="460px"
      >
        {createdKey ? (
          <div className="settings-key-created">
            <p className="settings-key-created__warning">
              Copy this key now — it won't be shown again.
            </p>
            <div className="settings-key-created__value" onClick={copyKey}>
              <code>{createdKey}</code>
              <span className="settings-key-created__copy">📋</span>
            </div>
            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: "var(--space-4)" }}>
              <Button onClick={() => setShowCreate(false)}>Done</Button>
            </div>
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-4)" }}>
            <Input
              label="Key Name"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              placeholder="e.g. paperback"
            />
            <div style={{ display: "flex", justifyContent: "flex-end", gap: "var(--space-3)" }}>
              <Button variant="ghost" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button onClick={handleCreate} loading={creating}>Create</Button>
            </div>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete API Key"
        message={deleteTarget ? `Delete API key "${deleteTarget.name}"? Any integrations using this key will stop working.` : ""}
        confirmLabel="Delete"
        danger
      />

      <ConfirmDialog
        open={!!restoreTarget}
        onClose={() => setRestoreTarget(null)}
        onConfirm={handleRestoreBackup}
        title="Restore Backup"
        message={restoreTarget ? `Restore "${restoreTarget.name}"? This will replace current manga, chapters, collections, discover sections, and chapter sources.` : ""}
        confirmLabel="Restore"
        danger
      />
    </div>
  );
}
