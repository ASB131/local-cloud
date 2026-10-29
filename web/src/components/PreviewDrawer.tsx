import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  downloadFile,
  getFile,
  issueMediaToken,
  issueVersionMediaToken,
  listFileVersions,
  restoreFileVersion,
} from "@/api/files";
import type { FileVersion } from "@/api/types";
import type { CloudFile } from "@/api/types";
import { ExplorerFileIcon } from "@/components/ExplorerFileIcon";
import { FilePreviewContent, type TextViewMode } from "@/components/FilePreviewContent";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { fileTypeLabel } from "@/lib/fileIcon";
import { formatBytes, formatDateTime } from "@/lib/formatBytes";
import { replaceFileContent } from "@/lib/replaceFileContent";
import {
  isCsvFile,
  isEditableTextFile,
  isMarkdownFile,
  PREVIEW_TEXT_MAX_BYTES,
  previewKind,
} from "@/lib/previewKind";
import { contentUrl, getMediaToken, getVersionMediaToken, versionContentUrl } from "@/media/previewUrl";

export type PreviewDrawerActions = {
  onRename: (file: CloudFile) => void;
  onDelete: (file: CloudFile) => void;
  onFavorite: (file: CloudFile) => void;
};

type Props = {
  file: CloudFile | null;
  folderFiles: CloudFile[];
  folderPathLabel: string;
  onFileChange: (file: CloudFile) => void;
  onClose: () => void;
  actions: PreviewDrawerActions;
};

type ImageZoom = "fit" | "width" | "actual" | number;

function ToolbarButton({
  label,
  title,
  disabled,
  active,
  onClick,
  children,
}: {
  label: string;
  title?: string;
  disabled?: boolean;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={`preview-tool-btn${active ? " is-active" : ""}`}
      aria-label={label}
      title={title ?? label}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function PreviewDrawer({ file, folderFiles, folderPathLabel, onFileChange, onClose, actions }: Props) {
  const qc = useQueryClient();
  const viewportRef = useRef<HTMLDivElement>(null);
  const [src, setSrc] = useState<string | null>(null);
  const [imageZoom, setImageZoom] = useState<ImageZoom>("fit");
  const [textWrap, setTextWrap] = useState(true);
  const [copied, setCopied] = useState<string | null>(null);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [contentRev, setContentRev] = useState(0);
  const [draft, setDraft] = useState("");
  const [savedBaseline, setSavedBaseline] = useState("");
  const [saveError, setSaveError] = useState<string | null>(null);
  const [textViewMode, setTextViewMode] = useState<TextViewMode>("source");
  const [viewVersionId, setViewVersionId] = useState<string | null>(null);

  const fileIndex = useMemo(() => (file ? folderFiles.findIndex((f) => f.id === file.id) : -1), [file, folderFiles]);
  const hasPrev = fileIndex > 0;
  const hasNext = fileIndex >= 0 && fileIndex < folderFiles.length - 1;

  const goPrev = useCallback(() => {
    if (!hasPrev) return;
    onFileChange(folderFiles[fileIndex - 1]!);
  }, [fileIndex, folderFiles, hasPrev, onFileChange]);

  const goNext = useCallback(() => {
    if (!hasNext) return;
    onFileChange(folderFiles[fileIndex + 1]!);
  }, [fileIndex, folderFiles, hasNext, onFileChange]);

  const fileDetail = useQuery({
    queryKey: ["file", file?.id],
    queryFn: () => getFile(file!.id),
    enabled: Boolean(file?.id),
  });

  const versions = useQuery({
    queryKey: ["file", file?.id, "versions"],
    queryFn: () => listFileVersions(file!.id),
    enabled: Boolean(file?.id),
  });

  const displayFile = fileDetail.data ?? file;

  const currentVersionNum = useMemo(() => {
    const items = versions.data ?? [];
    if (items.length === 0) return 0;
    return Math.max(...items.map((v) => v.version_num));
  }, [versions.data]);

  const restoreMut = useMutation({
    mutationFn: (versionId: string) => restoreFileVersion(file!.id, versionId),
    onSuccess: () => {
      setViewVersionId(null);
      qc.invalidateQueries({ queryKey: ["file", file?.id, "versions"] });
      qc.invalidateQueries({ queryKey: ["folder"] });
      setContentRev((v) => v + 1);
    },
  });

  const saveMut = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error("No file");
      const byteLen = new TextEncoder().encode(draft).length;
      if (byteLen > PREVIEW_TEXT_MAX_BYTES) {
        throw new Error(
          `Edited file is too large (${Math.round(byteLen / (1024 * 1024))} MiB). Save limit is 5 MiB in the browser.`,
        );
      }
      const mime =
        file.mime_type && file.mime_type !== "application/octet-stream"
          ? file.mime_type
          : "text/plain;charset=utf-8";
      await replaceFileContent(file, new Blob([draft], { type: mime }));
      return getFile(file.id);
    },
    onSuccess: (updated) => {
      setSaveError(null);
      setSavedBaseline(draft);
      setContentRev((v) => v + 1);
      qc.invalidateQueries({ queryKey: ["folder"] });
      qc.invalidateQueries({ queryKey: ["file", file?.id, "versions"] });
      onFileChange(updated);
    },
    onError: (err: Error) => {
      setSaveError(err.message || "Could not save file.");
    },
  });

  const activeVersion = useMemo((): FileVersion | null => {
    if (!viewVersionId) return null;
    return (versions.data ?? []).find((v) => v.id === viewVersionId) ?? null;
  }, [viewVersionId, versions.data]);

  const fileForPreview = useMemo(() => {
    if (!displayFile) return null;
    if (!activeVersion) return displayFile;
    return { ...displayFile, size_bytes: activeVersion.size_bytes, file_hash: activeVersion.file_hash };
  }, [displayFile, activeVersion]);

  useEffect(() => {
    if (!file) {
      setSrc(null);
      return;
    }
    setImageZoom("fit");
    let cancelled = false;
    (async () => {
      if (viewVersionId) {
        const token = await getVersionMediaToken(file.id, viewVersionId, (fid, vid) =>
          issueVersionMediaToken(fid, vid),
        );
        if (cancelled) return;
        setSrc(versionContentUrl(file.id, viewVersionId, token));
        return;
      }
      const token = await getMediaToken(file.id, issueMediaToken);
      if (cancelled) return;
      setSrc(contentUrl(file.id, token));
    })().catch(() => {
      if (!cancelled) setSrc(null);
    });
    return () => {
      cancelled = true;
    };
  }, [file, contentRev, viewVersionId]);

  useEffect(() => {
    setDraft("");
    setSavedBaseline("");
    setSaveError(null);
    setTextViewMode("source");
    setInspectorOpen(false);
    setViewVersionId(null);
  }, [file?.id]);

  const dirty = draft !== savedBaseline;

  useEffect(() => {
    if (!dirty) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  const confirmLoseChanges = useCallback(() => {
    if (!dirty) return true;
    return window.confirm("Discard unsaved changes?");
  }, [dirty]);

  const requestClose = useCallback(() => {
    if (!confirmLoseChanges()) return;
    onClose();
  }, [confirmLoseChanges, onClose]);

  const guardedGoPrev = useCallback(() => {
    if (!confirmLoseChanges()) return;
    goPrev();
  }, [confirmLoseChanges, goPrev]);

  const guardedGoNext = useCallback(() => {
    if (!confirmLoseChanges()) return;
    goNext();
  }, [confirmLoseChanges, goNext]);

  const handleTextLoaded = useCallback((baseline: string) => {
    setSavedBaseline(baseline);
    setDraft(baseline);
  }, []);

  const handleSave = useCallback(() => {
    if (saveMut.isPending || draft === savedBaseline) return;
    saveMut.mutate();
  }, [saveMut, draft, savedBaseline]);

  useEffect(() => {
    if (!file) return;
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      const inField = tag === "INPUT" || tag === "TEXTAREA";
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s" && isEditableTextFile(file)) {
        e.preventDefault();
        handleSave();
        return;
      }
      if (inField) return;
      if (e.key === "Escape") {
        e.preventDefault();
        requestClose();
        return;
      }
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        guardedGoPrev();
        return;
      }
      if (e.key === "ArrowRight") {
        e.preventDefault();
        guardedGoNext();
        return;
      }
      if (e.key === "F2") {
        e.preventDefault();
        if (!confirmLoseChanges()) return;
        actions.onRename(file);
        return;
      }
      if (e.key === "Delete") {
        e.preventDefault();
        if (!confirmLoseChanges()) return;
        actions.onDelete(file);
        onClose();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
    };
  }, [file, dirty, requestClose, guardedGoPrev, guardedGoNext, actions, onClose, confirmLoseChanges, handleSave]);

  const copyBrief = async (label: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      window.setTimeout(() => setCopied(null), 2000);
    } catch {
      /* ignore */
    }
  };

  const toggleFullscreen = () => {
    const el = viewportRef.current;
    if (!el) return;
    if (document.fullscreenElement) void document.exitFullscreen();
    else void el.requestFullscreen();
  };

  if (!file) return null;

  const kind = previewKind(file);
  const isImage = kind === "image";
  const isMedia = isImage || kind === "video" || kind === "audio";
  const isEmbedViewport = kind === "pdf" || kind === "office";
  const canEdit = isEditableTextFile(file) && !viewVersionId;
  const showMarkdown = canEdit && isMarkdownFile(file.name, file.mime_type);
  const showCsvTable = canEdit && isCsvFile(file.name, file.mime_type);
  const textEdit = canEdit
    ? {
        draft,
        viewMode: textViewMode,
        onDraftChange: setDraft,
        onLoaded: handleTextLoaded,
        onSave: handleSave,
      }
    : undefined;
  const positionLabel =
    fileIndex >= 0 && folderFiles.length > 0 ? `${fileIndex + 1} of ${folderFiles.length} in folder` : null;

  const detailRows: [string, string, string?][] = [
    ["Type", fileTypeLabel(file)],
    ["MIME", file.mime_type || "—"],
    ["Size", formatBytes(file.size_bytes)],
    ["Uploaded", formatDateTime(file.created_at)],
    ["Modified", formatDateTime(file.updated_at)],
    ["File created", formatDateTime(file.meta_created_at)],
    ["File modified", formatDateTime(file.meta_modified_at)],
    ["Location", folderPathLabel],
    ["Favorite", file.is_favorite ? "Yes" : "No"],
  ];

  return (
    <div
      className="preview-overlay"
      role="dialog"
      aria-modal="true"
      aria-label={`Preview ${file.name}`}
      onClick={requestClose}
    >
      <div
        className="preview-shell"
        onClick={(e) => e.stopPropagation()}
        onWheel={(e) => e.stopPropagation()}
      >
        <header className="preview-toolbar">
          <div className="preview-toolbar-start">
            <ToolbarButton label="Close preview" onClick={requestClose}>
              ✕
            </ToolbarButton>
            <span className="preview-toolbar-divider" aria-hidden />
            <ToolbarButton label="Previous file" disabled={!hasPrev} onClick={guardedGoPrev}>
              ‹
            </ToolbarButton>
            <ToolbarButton label="Next file" disabled={!hasNext} onClick={guardedGoNext}>
              ›
            </ToolbarButton>
            {positionLabel ? <span className="preview-position">{positionLabel}</span> : null}
          </div>

          <div className="preview-toolbar-center">
            <span className="preview-toolbar-filename" title={file.name}>
              {fileDisplayName(file.name)}
              {dirty ? <span className="preview-unsaved-badge">Unsaved</span> : null}
            </span>
            {saveError ? <span className="preview-save-error">{saveError}</span> : null}
          </div>

          <div className="preview-toolbar-end">
            {isImage ? (
              <>
                <ToolbarButton
                  label="Fit to window"
                  active={imageZoom === "fit"}
                  onClick={() => setImageZoom("fit")}
                >
                  Fit
                </ToolbarButton>
                <ToolbarButton
                  label="Fit width"
                  active={imageZoom === "width"}
                  onClick={() => setImageZoom("width")}
                >
                  Width
                </ToolbarButton>
                <ToolbarButton
                  label="Actual size"
                  active={imageZoom === "actual"}
                  onClick={() => setImageZoom("actual")}
                >
                  100%
                </ToolbarButton>
                <ToolbarButton
                  label="Zoom in"
                  onClick={() =>
                    setImageZoom((z) => {
                      const base = typeof z === "number" ? z : 1;
                      return Math.min(base + 0.25, 4);
                    })
                  }
                >
                  +
                </ToolbarButton>
                <ToolbarButton
                  label="Zoom out"
                  onClick={() =>
                    setImageZoom((z) => {
                      const base = typeof z === "number" ? z : 1;
                      return Math.max(base - 0.25, 0.25);
                    })
                  }
                >
                  −
                </ToolbarButton>
                <span className="preview-toolbar-divider" aria-hidden />
              </>
            ) : null}
            {kind === "text" ? (
              <>
                {canEdit ? (
                  <>
                    <ToolbarButton
                      label="Save"
                      active
                      disabled={!dirty || saveMut.isPending}
                      onClick={handleSave}
                    >
                      {saveMut.isPending ? "Saving…" : "Save"}
                    </ToolbarButton>
                    <ToolbarButton
                      label="Discard changes"
                      disabled={!dirty || saveMut.isPending}
                      onClick={() => {
                        if (!window.confirm("Discard unsaved changes?")) return;
                        setDraft(savedBaseline);
                        setSaveError(null);
                      }}
                    >
                      Discard
                    </ToolbarButton>
                    <ToolbarButton label="Copy all" onClick={() => void copyBrief("text", draft)}>
                      Copy
                    </ToolbarButton>
                    {showMarkdown ? (
                      <>
                        <ToolbarButton
                          label="Markdown source"
                          active={textViewMode === "source"}
                          onClick={() => setTextViewMode("source")}
                        >
                          Source
                        </ToolbarButton>
                        <ToolbarButton
                          label="Markdown preview"
                          active={textViewMode === "markdown"}
                          onClick={() => setTextViewMode("markdown")}
                        >
                          Preview
                        </ToolbarButton>
                      </>
                    ) : null}
                    {showCsvTable ? (
                      <>
                        <ToolbarButton
                          label="Edit as text"
                          active={textViewMode === "source"}
                          onClick={() => setTextViewMode("source")}
                        >
                          Text
                        </ToolbarButton>
                        <ToolbarButton
                          label="View as table"
                          active={textViewMode === "csv-table"}
                          onClick={() => setTextViewMode("csv-table")}
                        >
                          Table
                        </ToolbarButton>
                      </>
                    ) : null}
                  </>
                ) : null}
                <ToolbarButton
                  label="Toggle word wrap"
                  active={textWrap}
                  onClick={() => setTextWrap((v) => !v)}
                >
                  Wrap
                </ToolbarButton>
                <span className="preview-toolbar-divider" aria-hidden />
              </>
            ) : null}
            <div className="preview-toolbar-actions" role="group" aria-label="File actions">
              <ToolbarButton
                label={file.is_favorite ? "Remove from favorites" : "Add to favorites"}
                active={file.is_favorite}
                onClick={() => actions.onFavorite(file)}
              >
                {file.is_favorite ? "★" : "☆"}
              </ToolbarButton>
              <ToolbarButton label="Rename" onClick={() => actions.onRename(file)}>
                Rename
              </ToolbarButton>
              <span className="preview-toolbar-divider" aria-hidden />
              <ToolbarButton
                label="Open in new tab"
                disabled={!src}
                onClick={() => src && window.open(src, "_blank", "noopener,noreferrer")}
              >
                Open
              </ToolbarButton>
              <ToolbarButton label="Download" onClick={() => void downloadFile(file.id, file.name)}>
                Download
              </ToolbarButton>
              {isMedia ? (
                <>
                  <span className="preview-toolbar-divider" aria-hidden />
                  <ToolbarButton label="Fullscreen" onClick={toggleFullscreen}>
                    Fullscreen
                  </ToolbarButton>
                </>
              ) : null}
              <span className="preview-toolbar-divider" aria-hidden />
              <ToolbarButton
                label={inspectorOpen ? "Hide details panel" : "Show details panel"}
                active={inspectorOpen}
                onClick={() => setInspectorOpen((v) => !v)}
              >
                Details
              </ToolbarButton>
            </div>
          </div>
        </header>

        {viewVersionId && activeVersion ? (
          <div className="preview-version-banner">
            <span>
              Viewing older version <strong>v{activeVersion.version_num}</strong> (read-only)
            </span>
            <button type="button" className="explorer-btn" onClick={() => setViewVersionId(null)}>
              Back to current
            </button>
          </div>
        ) : null}

        <div className={`preview-workspace${inspectorOpen ? "" : " preview-workspace--wide"}`}>
          <div
            ref={viewportRef}
            className={`preview-viewport${isMedia ? " preview-viewport--media" : " preview-viewport--doc"}${isEmbedViewport ? " preview-viewport--embed" : ""}`}
          >
            <FilePreviewContent
              file={fileForPreview ?? file}
              contentUrl={src}
              imageZoom={isImage ? imageZoom : undefined}
              textWrap={textWrap}
              textEdit={textEdit}
              onCopyText={undefined}
            />
          </div>

          {inspectorOpen ? (
            <aside className="preview-inspector" aria-label="File details">
              <div className="preview-inspector-hero">
                <div className="preview-inspector-icon">
                  <ExplorerFileIcon file={file} variant="grid" />
                </div>
                <h2 className="preview-inspector-name" title={file.name}>
                  {fileDisplayName(file.name)}
                </h2>
                <p className="preview-inspector-sub">
                  {fileTypeLabel(file)} · {formatBytes(file.size_bytes)}
                </p>
              </div>

              <div className="preview-inspector-actions">
                <button type="button" className="explorer-btn explorer-btn-primary" onClick={() => void downloadFile(file.id, file.name)}>
                  Download
                </button>
                <button type="button" className="explorer-btn" onClick={() => actions.onRename(file)}>
                  Rename
                </button>
                <button type="button" className="explorer-btn" onClick={() => actions.onDelete(file)}>
                  Delete
                </button>
              </div>

              {copied ? <p className="preview-copy-toast">Copied {copied} to clipboard</p> : null}

              <section className="preview-inspector-section">
                <h3>Details</h3>
                <dl className="preview-details">
                  {detailRows.map(([k, v]) => (
                    <div key={k} className="preview-details-row">
                      <dt>{k}</dt>
                      <dd title={v}>{v}</dd>
                    </div>
                  ))}
                  <div className="preview-details-row">
                    <dt>Hash</dt>
                    <dd className="preview-hash">
                      <code title={file.file_hash}>{file.file_hash.slice(0, 16)}…</code>
                      <button type="button" className="preview-link-btn" onClick={() => void copyBrief("hash", file.file_hash)}>
                        Copy
                      </button>
                    </dd>
                  </div>
                </dl>
              </section>

              <section className="preview-inspector-section">
                <h3>Versions</h3>
                <p className="preview-muted preview-version-note">Up to 10 versions are kept when you save or replace a file.</p>
                {versions.isLoading ? <p className="preview-muted">Loading…</p> : null}
                {(versions.data?.length ?? 0) === 0 ? (
                  <p className="preview-muted">No version history yet.</p>
                ) : (
                  <ul className="preview-version-list">
                    {(versions.data ?? []).map((v) => {
                      const isCurrent = v.version_num === currentVersionNum && !viewVersionId;
                      const isViewing = viewVersionId === v.id;
                      return (
                        <li key={v.id} className={`preview-version-row${isViewing ? " is-viewing" : ""}`}>
                          <div className="preview-version-meta">
                            <strong>v{v.version_num}</strong>
                            {isCurrent ? <span className="preview-version-badge">Current</span> : null}
                            {isViewing ? <span className="preview-version-badge">Previewing</span> : null}
                            <span>
                              {formatBytes(v.size_bytes)} · {formatDateTime(v.created_at)}
                            </span>
                          </div>
                          <div className="preview-version-actions">
                            {!isCurrent || viewVersionId ? (
                              <button
                                type="button"
                                className="explorer-btn"
                                onClick={() => {
                                  if (v.version_num === currentVersionNum) setViewVersionId(null);
                                  else setViewVersionId(v.id);
                                }}
                              >
                                {isViewing ? "Viewing" : "Preview"}
                              </button>
                            ) : null}
                            {v.version_num !== currentVersionNum ? (
                              <button
                                type="button"
                                className="explorer-btn"
                                disabled={restoreMut.isPending}
                                onClick={() => restoreMut.mutate(v.id)}
                              >
                                Restore
                              </button>
                            ) : null}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
                {restoreMut.isError ? <p className="preview-version-error">Could not restore that version.</p> : null}
              </section>
            </aside>
          ) : null}
        </div>
      </div>
    </div>
  );
}
