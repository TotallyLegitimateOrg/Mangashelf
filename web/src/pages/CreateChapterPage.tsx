import {
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
  type FormEvent,
} from "react";
import { useParams, useNavigate } from "react-router";
import * as api from "@/lib/api";
import { emptyChapterPayload, PROVIDERS, IMPORT_MODES } from "@/lib/types";
import type {
  ChapterPayload,
  ChapterImportPayload,
  ProviderDefinition,
} from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { UrlListEditor } from "@/components/ui/UrlListEditor";
import { useToast } from "@/components/ui/Toast";
import "./ChapterFormPage.css";

type UploadChapterMetadata = Omit<ChapterPayload, "pages">;

interface UploadFileEntry {
  id: string;
  file: File;
  metadata: UploadChapterMetadata;
}

interface UploadProgressState extends api.UploadChapterProgress {
  fileId: string;
  archiveName: string;
  fileIndex: number;
  totalFiles: number;
}

interface ChapterMetadataFieldsProps {
  metadata: UploadChapterMetadata;
  onChange: (patch: Partial<UploadChapterMetadata>) => void;
  chapterNumberRequired?: boolean;
}

interface UploadMetadataRowProps {
  metadata: UploadChapterMetadata;
  onChange: (patch: Partial<UploadChapterMetadata>) => void;
  chapterNumberRequired?: boolean;
  includeChapterNumber?: boolean;
  includeTitle?: boolean;
}

type ImportMode = ChapterImportPayload["mode"];

const LANGUAGE_OPTIONS = [
  { value: "", label: "Language" },
  { value: "AR", label: "Arabic" },
  { value: "EN", label: "English" },
];

const ACCEPTED_ARCHIVE_EXTENSIONS = new Set([".cbz", ".zip"]);

function createEmptyUploadMetadata(): UploadChapterMetadata {
  const { pages: _pages, ...metadata } = emptyChapterPayload();
  return metadata;
}

function createSharedUploadMetadata(): UploadChapterMetadata {
  return {
    ...createEmptyUploadMetadata(),
    langCode: "",
  };
}

function cloneUploadMetadata(
  metadata: UploadChapterMetadata
): UploadChapterMetadata {
  return {
    ...metadata,
    additionalInfo: metadata.additionalInfo.map((entry) => ({ ...entry })),
  };
}

function mergeUploadMetadata(
  baseMetadata: UploadChapterMetadata,
  parsedMetadata?: Partial<UploadChapterMetadata>
): UploadChapterMetadata {
  const merged = {
    ...cloneUploadMetadata(baseMetadata),
    ...parsedMetadata,
  };
  merged.additionalInfo = parsedMetadata?.additionalInfo
    ? parsedMetadata.additionalInfo.map((entry) => ({ ...entry }))
    : baseMetadata.additionalInfo.map((entry) => ({ ...entry }));
  return merged;
}

function createUploadEntry(
  file: File,
  baseMetadata: UploadChapterMetadata,
  parsedMetadata?: Partial<UploadChapterMetadata>
): UploadFileEntry {
  return {
    id: crypto.randomUUID(),
    file,
    metadata: mergeUploadMetadata(baseMetadata, parsedMetadata),
  };
}

function appendUploadFiles(
  existingEntries: UploadFileEntry[],
  files: File[],
  baseMetadata: UploadChapterMetadata
): UploadFileEntry[] {
  return [
    ...existingEntries,
    ...files.map((file) => createUploadEntry(file, baseMetadata)),
  ];
}

function applyMetadataToAll(
  entries: UploadFileEntry[],
  patch: Partial<UploadChapterMetadata>
): UploadFileEntry[] {
  return entries.map((entry) => ({
    ...entry,
    metadata: {
      ...entry.metadata,
      ...patch,
      additionalInfo: patch.additionalInfo
        ? patch.additionalInfo.map((info) => ({ ...info }))
        : entry.metadata.additionalInfo,
    },
  }));
}

function toNonEmptyUploadMetadataPatch(
  metadata: UploadChapterMetadata
): Partial<UploadChapterMetadata> {
  const patch: Partial<UploadChapterMetadata> = {};

  if (metadata.chapNum) {
    patch.chapNum = metadata.chapNum;
  }
  if (metadata.volume != null) {
    patch.volume = metadata.volume;
  }
  if (metadata.title.trim()) {
    patch.title = metadata.title;
  }
  if (metadata.version.trim()) {
    patch.version = metadata.version;
  }
  if (metadata.langCode.trim()) {
    patch.langCode = metadata.langCode;
  }
  if (metadata.additionalInfo.length > 0) {
    patch.additionalInfo = metadata.additionalInfo.map((entry) => ({ ...entry }));
  }

  return patch;
}

function appendUploadMetadataToFormData(
  formData: FormData,
  metadata: UploadChapterMetadata
) {
  formData.append("langCode", metadata.langCode);
  formData.append("chapNum", String(metadata.chapNum));
  formData.append("title", metadata.title);
  formData.append("version", metadata.version);
  if (metadata.volume != null) {
    formData.append("volume", String(metadata.volume));
  }
  if (metadata.publishDate) {
    formData.append("publishDate", metadata.publishDate);
  }
  if (metadata.creationDate) {
    formData.append("creationDate", metadata.creationDate);
  }
  if (metadata.sortingIndex != null) {
    formData.append("sortingIndex", String(metadata.sortingIndex));
  }
  if (metadata.additionalInfo.length > 0) {
    formData.append("additionalInfo", JSON.stringify(metadata.additionalInfo));
  }
}

function getArchiveExtension(fileName: string): string {
  const parts = fileName.toLowerCase().split(".");
  if (parts.length < 2) {
    return "";
  }
  return `.${parts.at(-1)}`;
}

function isSupportedArchive(file: File): boolean {
  return ACCEPTED_ARCHIVE_EXTENSIONS.has(getArchiveExtension(file.name));
}

function formatFileSize(sizeInBytes: number): string {
  return `${(sizeInBytes / 1024 / 1024).toFixed(1)} MB`;
}

function formatUploadProgressLabel(progress: UploadProgressState): string {
  switch (progress.phase) {
    case "uploading_archive": {
      if (progress.loadedBytes != null && progress.totalBytes) {
        const percent = Math.min(
          100,
          Math.round((progress.loadedBytes / progress.totalBytes) * 100)
        );
        return `Uploading archive... ${percent}%`;
      }
      return "Uploading archive...";
    }
    case "extracting_archive":
      if (progress.current != null && progress.total) {
        return `Extracting archive... ${progress.current} of ${progress.total}`;
      }
      return "Extracting archive...";
    case "uploading_images":
      if (progress.current != null && progress.total) {
        return `Uploading extracted images... ${progress.current} of ${progress.total}`;
      }
      return "Uploading extracted images...";
    case "creating_chapter":
      return "Saving chapter...";
    default:
      return progress.message ?? "Processing upload...";
  }
}

function formatUploadProgressDetail(progress: UploadProgressState): string {
  if (progress.phase === "uploading_archive") {
    const uploadedBytes = progress.loadedBytes ?? 0;
    const totalBytes = progress.totalBytes ?? uploadedBytes;
    return `${formatFileSize(uploadedBytes)} of ${formatFileSize(totalBytes)}`;
  }
  if (progress.fileName) {
    return progress.fileName;
  }
  return progress.archiveName;
}

function parseRequiredNumber(value: string): number {
  return parseFloat(value) || 0;
}

function parseOptionalNumber(value: string): number | null {
  return value ? parseFloat(value) : null;
}

function ChapterMetadataFields({
  metadata,
  onChange,
  chapterNumberRequired = false,
}: ChapterMetadataFieldsProps) {
  return (
    <div className="chapter-form__fields">
      <Input
        label="Title"
        value={metadata.title}
        onChange={(e) => onChange({ title: e.target.value })}
        placeholder="Chapter title"
      />
      <div className="chapter-form__row">
        <Input
          label="Chapter Number"
          type="number"
          step="any"
          value={metadata.chapNum || ""}
          onChange={(e) =>
            onChange({ chapNum: parseFloat(e.target.value) || 0 })
          }
          required={chapterNumberRequired}
        />
        <Input
          label="Volume"
          type="number"
          step="any"
          value={metadata.volume ?? ""}
          onChange={(e) =>
            onChange({
              volume: e.target.value ? parseFloat(e.target.value) : null,
            })
          }
        />
      </div>
      <div className="chapter-form__row">
        <Select
          label="Language"
          value={metadata.langCode}
          onChange={(e) => onChange({ langCode: e.target.value })}
          options={LANGUAGE_OPTIONS}
        />
        <Input
          label="Version"
          value={metadata.version}
          onChange={(e) => onChange({ version: e.target.value })}
          placeholder="e.g. Group name"
        />
      </div>
    </div>
  );
}

function UploadMetadataRow({
  metadata,
  onChange,
  chapterNumberRequired = false,
  includeChapterNumber = true,
  includeTitle = true,
}: UploadMetadataRowProps) {
  return (
    <div
      className={`chapter-form__upload-row ${includeChapterNumber && includeTitle ? "" : "chapter-form__upload-row--defaults"}`}
    >
      {includeChapterNumber ? (
        <input
          className="chapter-form__upload-input chapter-form__upload-input--number"
          type="number"
          step="any"
          inputMode="decimal"
          value={metadata.chapNum || ""}
          onChange={(e) => onChange({ chapNum: parseRequiredNumber(e.target.value) })}
          placeholder="Ch."
          aria-label="Chapter number"
          required={chapterNumberRequired}
        />
      ) : null}
      <input
        className="chapter-form__upload-input chapter-form__upload-input--number"
        type="number"
        step="any"
        inputMode="decimal"
        value={metadata.volume ?? ""}
        onChange={(e) => onChange({ volume: parseOptionalNumber(e.target.value) })}
        placeholder="Vol."
        aria-label="Volume"
      />
      {includeTitle ? (
        <input
          className="chapter-form__upload-input"
          value={metadata.title}
          onChange={(e) => onChange({ title: e.target.value })}
          placeholder="Title"
          aria-label="Title"
        />
      ) : null}
      <input
        className="chapter-form__upload-input"
        value={metadata.version}
        onChange={(e) => onChange({ version: e.target.value })}
        placeholder="Version"
        aria-label="Version"
      />
      <div className="chapter-form__upload-select-wrap">
        <select
          className="chapter-form__upload-select"
          value={metadata.langCode}
          onChange={(e) => onChange({ langCode: e.target.value })}
          aria-label="Language"
        >
          {LANGUAGE_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

export default function CreateChapterPage() {
  const { id: mangaId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [form, setForm] = useState<ChapterPayload>(emptyChapterPayload());
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<"manual" | "upload" | "remote">("manual");

  const [provider, setProvider] = useState<ProviderDefinition>(PROVIDERS[0]!);
  const [importMode, setImportMode] = useState<ImportMode>("import_once");
  const [providerConfig, setProviderConfig] = useState<Record<string, string>>(
    {}
  );
  const [syncInterval, setSyncInterval] = useState(60);

  const [uploadTemplate, setUploadTemplate] = useState<UploadChapterMetadata>(
    createSharedUploadMetadata()
  );
  const [files, setFiles] = useState<UploadFileEntry[]>([]);
  const [uploadProgress, setUploadProgress] = useState<UploadProgressState | null>(null);
  const [isDraggingFiles, setIsDraggingFiles] = useState(false);
  const dragDepthRef = useRef(0);

  const update = <K extends keyof ChapterPayload>(
    key: K,
    val: ChapterPayload[K]
  ) => {
    setForm((prev) => ({ ...prev, [key]: val }));
  };

  const updateUploadTemplate = (patch: Partial<UploadChapterMetadata>) => {
    setUploadTemplate((prev) => ({ ...prev, ...patch }));
  };

  const updateFileEntry = (
    id: string,
    patch: Partial<UploadChapterMetadata>
  ) => {
    setFiles((prev) =>
      prev.map((entry) =>
        entry.id === id
          ? { ...entry, metadata: { ...entry.metadata, ...patch } }
          : entry
      )
    );
  };

  const removeFileEntry = (id: string) => {
    setFiles((prev) => prev.filter((entry) => entry.id !== id));
  };

  const queueFiles = (incomingFiles: File[]) => {
    if (incomingFiles.length === 0) {
      return;
    }
    const validFiles = incomingFiles.filter(isSupportedArchive);
    const invalidCount = incomingFiles.length - validFiles.length;

    if (invalidCount > 0) {
      toast(
        `Ignored ${invalidCount} unsupported file${invalidCount !== 1 ? "s" : ""}. Only CBZ and ZIP are allowed.`,
        "error"
      );
    }
    if (validFiles.length === 0) {
      return;
    }

    const sharedPatch = toNonEmptyUploadMetadataPatch(uploadTemplate);
    const baseMetadata = {
      ...createEmptyUploadMetadata(),
      ...sharedPatch,
    };
    setFiles((prev) => appendUploadFiles(prev, validFiles, baseMetadata));
  };

  const handleFilesSelected = (e: ChangeEvent<HTMLInputElement>) => {
    queueFiles(Array.from(e.target.files ?? []));
    e.target.value = "";
  };

  const handleDragEnter = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current += 1;
    setIsDraggingFiles(true);
  };

  const handleDragOver = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    if (!isDraggingFiles) {
      setIsDraggingFiles(true);
    }
  };

  const handleDragLeave = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) {
      setIsDraggingFiles(false);
    }
  };

  const handleDrop = (e: DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current = 0;
    setIsDraggingFiles(false);
    queueFiles(Array.from(e.dataTransfer.files ?? []));
  };

  const handleApplyToAll = () => {
    if (files.length === 0) {
      toast("Add files before applying metadata", "error");
      return;
    }
    const patch = toNonEmptyUploadMetadataPatch(uploadTemplate);
    if (Object.keys(patch).length === 0) {
      toast("Fill at least one shared field first", "error");
      return;
    }
    setFiles((prev) => applyMetadataToAll(prev, patch));
    toast(
      `Applied metadata to ${files.length} file${files.length !== 1 ? "s" : ""}`,
      "success"
    );
  };

  const handleProviderChange = (providerId: string) => {
    const found = PROVIDERS.find((p) => p.id === providerId);
    if (found) {
      setProvider(found);
      setProviderConfig({});
    }
  };

  const updateProviderConfig = (key: string, value: string) => {
    setProviderConfig((prev) => ({ ...prev, [key]: value }));
  };

  const validateRemoteConfig = (): boolean => {
    for (const field of provider.configFields) {
      if (field.required && !providerConfig[field.key]?.trim()) {
        toast(`${field.label} is required`, "error");
        return false;
      }
    }
    return true;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!mangaId) {
      return;
    }

    setLoading(true);
    try {
      if (mode === "remote") {
        if (!validateRemoteConfig()) {
          setLoading(false);
          return;
        }
        const payload: ChapterImportPayload = {
          provider: provider.id,
          mode: importMode,
          config: { ...providerConfig },
          ...(importMode === "sync"
            ? { syncIntervalMinutes: syncInterval }
            : {}),
        };
        const result = await api.createChapterImport(mangaId, payload);

        if (importMode === "import_once") {
          toast(
            `Imported ${result.insertedCount} new chapter${result.insertedCount !== 1 ? "s" : ""} (${result.skippedCount} skipped)`,
            "success"
          );
        } else if (importMode === "proxy") {
          toast("Source linked in proxy mode!", "success");
        } else {
          toast(
            `Synced: ${result.insertedCount} inserted, ${result.updatedCount} updated, ${result.unchangedCount} unchanged, ${result.skippedCount} skipped`,
            "success"
          );
        }
      } else if (mode === "upload") {
        if (files.length === 0) {
          toast("Select at least one file", "error");
          setLoading(false);
          return;
        }

        for (const entry of files) {
          if (!entry.metadata.chapNum) {
            toast(
              `Chapter number is required for "${entry.file.name}"`,
              "error"
            );
            setLoading(false);
            return;
          }
        }

        for (const [i, entry] of files.entries()) {
          setUploadProgress({
            fileId: entry.id,
            archiveName: entry.file.name,
            fileIndex: i + 1,
            totalFiles: files.length,
            phase: "uploading_archive",
            loadedBytes: 0,
            totalBytes: entry.file.size,
          });
          const fd = new FormData();
          fd.append("file", entry.file);
          appendUploadMetadataToFormData(fd, entry.metadata);
          await api.uploadChapter(mangaId, fd, {
            onProgress: (progress) =>
              setUploadProgress({
                ...progress,
                fileId: entry.id,
                archiveName: entry.file.name,
                fileIndex: i + 1,
                totalFiles: files.length,
                totalBytes:
                  progress.phase === "uploading_archive"
                    ? (progress.totalBytes ?? entry.file.size)
                    : progress.totalBytes,
              }),
          });
        }
        setUploadProgress(null);
        toast(
          `${files.length} chapter${files.length !== 1 ? "s" : ""} uploaded!`,
          "success"
        );
      } else {
        await api.createChapter(mangaId, form);
        toast("Chapter created!", "success");
      }
      navigate(`/manga/${mangaId}`);
    } catch (err) {
      setUploadProgress(null);
      toast(
        api.errorMessage(err, "Failed to create chapter"),
        "error"
      );
    } finally {
      setLoading(false);
    }
  };

  const totalUploadSize = files.reduce((sum, entry) => sum + entry.file.size, 0);
  const hasQueuedFiles = files.length > 0;

  const submitLabel = () => {
    if (mode === "remote") {
      if (importMode === "import_once") {
        return "Import Chapters";
      }
      if (importMode === "proxy") {
        return "Link Source";
      }
      return "Add & Sync";
    }
    if (mode === "upload") {
      if (files.length === 0) {
        return "Upload Chapters";
      }
      return files.length === 1 ? "Upload 1 Chapter" : `Upload ${files.length} Chapters`;
    }
    return "Create Chapter";
  };

  return (
    <div>
      <div className="page-header">
        <div className="page-header__left">
          <button
            className="page-header__back"
            onClick={() => navigate(`/manga/${mangaId}`)}
          >
            ←
          </button>
          <h1 className="page-title">Add Chapter</h1>
        </div>
        <div className="page-header__actions">
          <Button
            type="submit"
            form="chapter-create-form"
            size="sm"
            loading={loading}
          >
            {submitLabel()}
          </Button>
        </div>
      </div>

      <div className="chapter-mode-toggle">
        <button
          className={`chapter-mode-toggle__btn ${mode === "manual" ? "chapter-mode-toggle__btn--active" : ""}`}
          type="button"
          onClick={() => setMode("manual")}
        >
          Manual
        </button>
        <button
          className={`chapter-mode-toggle__btn ${mode === "upload" ? "chapter-mode-toggle__btn--active" : ""}`}
          type="button"
          onClick={() => setMode("upload")}
        >
          Upload CBZ/ZIP
        </button>
        <button
          className={`chapter-mode-toggle__btn ${mode === "remote" ? "chapter-mode-toggle__btn--active" : ""}`}
          type="button"
          onClick={() => setMode("remote")}
        >
          Remote Import
        </button>
      </div>

      <form
        id="chapter-create-form"
        className={`chapter-form ${mode === "upload" ? "chapter-form--upload-mode" : ""}`}
        onSubmit={handleSubmit}
      >
        {mode === "remote" ? (
          <div className="remote-import">
            <div className="remote-import__row">
              <Select
                label="Provider"
                value={provider.id}
                onChange={(e) => handleProviderChange(e.target.value)}
                options={PROVIDERS.map((p) => ({
                  value: p.id,
                  label: p.label,
                }))}
              />
            </div>

            <div className="remote-import__section">
              <label className="input-group__label">Import Mode</label>
              <div className="chapter-mode-toggle">
                {IMPORT_MODES.map((m) => (
                  <button
                    key={m.value}
                    type="button"
                    className={`chapter-mode-toggle__btn ${importMode === m.value ? "chapter-mode-toggle__btn--active" : ""}`}
                    onClick={() => setImportMode(m.value)}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
              <p className="input-group__hint">
                {IMPORT_MODES.find((m) => m.value === importMode)?.description}
              </p>
            </div>

            <div className="remote-import__config">
              {provider.configFields.map((field) => (
                <Input
                  key={field.key}
                  label={field.label}
                  type={field.type === "url" ? "text" : field.type}
                  value={providerConfig[field.key] ?? ""}
                  onChange={(e) =>
                    updateProviderConfig(field.key, e.target.value)
                  }
                  placeholder={field.placeholder}
                  required={field.required}
                />
              ))}
            </div>

            {importMode === "sync" && (
              <div className="remote-import__sync-interval">
                <Input
                  label="Sync Interval (minutes)"
                  type="number"
                  value={syncInterval}
                  onChange={(e) =>
                    setSyncInterval(parseInt(e.target.value, 10) || 60)
                  }
                  hint="How often to check for new chapters"
                />
              </div>
            )}
          </div>
        ) : mode === "upload" ? (
          <div className="chapter-form__upload">
            <label
              className={`chapter-form__upload-area ${isDraggingFiles ? "chapter-form__upload-area--active" : ""}`}
              onDragEnter={handleDragEnter}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
            >
              <input
                type="file"
                accept=".cbz,.zip"
                multiple
                onChange={handleFilesSelected}
                className="sr-only"
              />
              <span className="chapter-form__upload-text">
                {hasQueuedFiles
                  ? "Drop more files here or click to add more."
                  : "Click or drag files here."}
              </span>
              <span className="chapter-form__upload-size">
                Supported types: .cbz, .zip
              </span>
            </label>

            {hasQueuedFiles ? (
              <section className="chapter-form__upload-batch">
                <div className="chapter-form__upload-batch-header">
                  <p className="chapter-form__upload-batch-summary">
                    {files.length} file{files.length !== 1 ? "s" : ""} ·{" "}
                    {formatFileSize(totalUploadSize)}
                  </p>
                  <div className="chapter-form__upload-shared">
                    <span className="chapter-form__upload-shared-label">
                      Shared
                    </span>
                    <div className="chapter-form__upload-shared-fields">
                      <UploadMetadataRow
                        metadata={uploadTemplate}
                        onChange={updateUploadTemplate}
                        includeChapterNumber={false}
                        includeTitle={false}
                      />
                    </div>
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      className="chapter-form__upload-shared-action"
                      onClick={handleApplyToAll}
                    >
                      Apply to all
                    </Button>
                  </div>
                </div>

                <div className="chapter-form__upload-file-list">
                  {files.map((entry) => (
                    <article key={entry.id} className="chapter-form__upload-item">
                      <div className="chapter-form__upload-item-header">
                        <div className="chapter-form__upload-file-meta">
                          <div className="chapter-form__upload-file-text">
                            <h3 className="chapter-form__upload-file-name">
                              {entry.file.name}
                            </h3>
                            <p className="chapter-form__upload-file-copy">
                              {formatFileSize(entry.file.size)}
                            </p>
                            {uploadProgress?.fileId === entry.id ? (
                              <p className="chapter-form__upload-file-status">
                                {formatUploadProgressLabel(uploadProgress)}
                              </p>
                            ) : null}
                          </div>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="chapter-form__upload-remove"
                          onClick={() => removeFileEntry(entry.id)}
                        >
                          Remove
                        </Button>
                      </div>
                      <div className="chapter-form__upload-item-fields">
                        <UploadMetadataRow
                          metadata={entry.metadata}
                          onChange={(patch) => updateFileEntry(entry.id, patch)}
                          chapterNumberRequired
                        />
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            ) : null}

            {uploadProgress && (
              <div className="chapter-form__progress" role="status" aria-live="polite">
                <p className="chapter-form__progress-title">
                  File {uploadProgress.fileIndex} of {uploadProgress.totalFiles}
                </p>
                <p className="chapter-form__progress-copy">{uploadProgress.archiveName}</p>
                <p className="chapter-form__progress-detail">
                  {formatUploadProgressLabel(uploadProgress)}
                </p>
                <p className="chapter-form__progress-meta">
                  {formatUploadProgressDetail(uploadProgress)}
                </p>
              </div>
            )}
          </div>
        ) : (
          <>
            <ChapterMetadataFields
              metadata={form}
              onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
              chapterNumberRequired
            />

            <UrlListEditor
              label="Page URLs"
              values={form.pages}
              onChange={(pages) => update("pages", pages)}
              hint="URLs in reading order"
            />
          </>
        )}
      </form>
    </div>
  );
}
