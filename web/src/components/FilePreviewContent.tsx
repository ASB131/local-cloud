import JSZip from "jszip";
import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import type { CloudFile } from "@/api/types";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { fileTypeLabel } from "@/lib/fileIcon";
import { formatBytes } from "@/lib/formatBytes";
import { renderMarkdownToHtml } from "@/lib/markdownRender";
import { parseCsvPreview } from "@/lib/parseCsvPreview";
import { HighlightedCodeBlock } from "@/components/HighlightedCodeBlock";
import { OfficePreviewPane } from "@/components/OfficePreviewPane";

const CodeMirrorTextEditor = lazy(() =>
  import("@/components/CodeMirrorTextEditor").then((m) => ({ default: m.CodeMirrorTextEditor })),
);
import { isSyntaxHighlightFile } from "@/lib/highlightCode";
import {
  PREVIEW_TEXT_MAX_BYTES,
  PREVIEW_ZIP_MAX_BYTES,
  isCsvFile,
  previewKind,
  textEditBaseline,
  type PreviewKind,
} from "@/lib/previewKind";

type ImageZoom = "fit" | "width" | "actual" | number;

export type TextViewMode = "source" | "markdown" | "csv-table";

export type TextEditState = {
  draft: string;
  viewMode: TextViewMode;
  onDraftChange: (value: string) => void;
  onLoaded: (baseline: string) => void;
  onSave: () => void;
};

type Props = {
  file: CloudFile;
  contentUrl: string | null;
  imageZoom?: ImageZoom;
  textWrap?: boolean;
  onCopyText?: (text: string) => void;
  textEdit?: TextEditState;
};

type ZipEntry = { path: string; size: number; dir: boolean };

export function FilePreviewContent({ file, contentUrl, imageZoom = "fit", textWrap = true, onCopyText, textEdit }: Props) {
  const kind = previewKind(file);
  const onTextLoadedRef = useRef(textEdit?.onLoaded);
  onTextLoadedRef.current = textEdit?.onLoaded;
  const [text, setText] = useState<string | null>(null);
  const [textError, setTextError] = useState<string | null>(null);
  const [zipEntries, setZipEntries] = useState<ZipEntry[] | null>(null);
  const [zipError, setZipError] = useState<string | null>(null);

  useEffect(() => {
    setText(null);
    setTextError(null);
    setZipEntries(null);
    setZipError(null);
    if (!contentUrl) return;

    if (kind === "text") {
      if (file.size_bytes > PREVIEW_TEXT_MAX_BYTES) {
        setTextError(`File is too large to preview (${Math.round(file.size_bytes / (1024 * 1024))} MiB). Download to open.`);
        return;
      }
      let cancelled = false;
      fetch(contentUrl)
        .then((r) => {
          if (!r.ok) throw new Error("fetch failed");
          return r.text();
        })
        .then((body) => {
          if (cancelled) return;
          const baseline = textEditBaseline(file.name, body);
          setText(baseline);
          onTextLoadedRef.current?.(baseline);
        })
        .catch(() => {
          if (!cancelled) setTextError("Could not load text preview.");
        });
      return () => {
        cancelled = true;
      };
    }

    if (kind === "zip") {
      if (file.size_bytes > PREVIEW_ZIP_MAX_BYTES) {
        setZipError("Archive is too large to list in the browser. Download to open.");
        return;
      }
      let cancelled = false;
      fetch(contentUrl)
        .then((r) => {
          if (!r.ok) throw new Error("fetch failed");
          return r.arrayBuffer();
        })
        .then(async (buf) => {
          const zip = await JSZip.loadAsync(buf);
          const list: ZipEntry[] = [];
          zip.forEach((path, entry) => {
            let size = 0;
            if (!entry.dir) {
              const data = (entry as JSZip.JSZipObject & { _data?: { uncompressedSize?: number } })._data;
              size = data?.uncompressedSize ?? 0;
            }
            list.push({ path, dir: entry.dir, size });
          });
          list.sort((a, b) => a.path.localeCompare(b.path));
          if (!cancelled) setZipEntries(list.slice(0, 2000));
        })
        .catch(() => {
          if (!cancelled) setZipError("Could not read ZIP archive.");
        });
      return () => {
        cancelled = true;
      };
    }
  }, [contentUrl, file, kind]);

  const csvRows = useMemo(() => {
    if (!text || !isCsvFile(file.name, file.mime_type)) return null;
    return parseCsvPreview(file.name, text);
  }, [text, file.name, file.mime_type]);

  if (!contentUrl) {
    return (
      <div className="preview-loading">
        <div className="preview-spinner" aria-hidden />
        <p>Loading preview…</p>
      </div>
    );
  }

  return (
    <PreviewBody
      kind={kind}
      file={file}
      contentUrl={contentUrl}
      imageZoom={imageZoom}
      textWrap={textWrap}
      onCopyText={onCopyText}
      textEdit={textEdit}
      text={text}
      textError={textError}
      csvRows={csvRows}
      zipEntries={zipEntries}
      zipError={zipError}
    />
  );
}

function PreviewBody({
  kind,
  file,
  contentUrl,
  imageZoom,
  textWrap,
  onCopyText,
  textEdit,
  text,
  textError,
  csvRows,
  zipEntries,
  zipError,
}: {
  kind: PreviewKind;
  file: CloudFile;
  contentUrl: string;
  imageZoom: ImageZoom;
  textWrap: boolean;
  onCopyText?: (text: string) => void;
  textEdit?: TextEditState;
  text: string | null;
  textError: string | null;
  csvRows: string[][] | null;
  zipEntries: ZipEntry[] | null;
  zipError: string | null;
}) {
  switch (kind) {
    case "image": {
      const zoomClass =
        imageZoom === "fit"
          ? "preview-image preview-image--fit"
          : imageZoom === "width"
            ? "preview-image preview-image--width"
            : imageZoom === "actual"
              ? "preview-image preview-image--actual"
              : "preview-image preview-image--scaled";
      const style = typeof imageZoom === "number" ? { transform: `scale(${imageZoom})` } : undefined;
      return (
        <div className={`preview-media-stage${imageZoom === "actual" || typeof imageZoom === "number" ? " preview-media-stage--scroll" : ""}`}>
          <img src={contentUrl} alt={file.name} className={zoomClass} style={style} />
        </div>
      );
    }
    case "video":
      return (
        <div className="preview-media-stage">
          <video src={contentUrl} controls className="preview-video" playsInline />
        </div>
      );
    case "audio":
      return (
        <div className="preview-audio-stage">
          <div className="preview-audio-visual" aria-hidden>
            <span className="preview-audio-glyph">♫</span>
          </div>
          <p className="preview-audio-title" title={file.name}>
            {fileDisplayName(file.name)}
          </p>
          <audio src={contentUrl} controls className="preview-audio" />
        </div>
      );
    case "pdf":
      return (
        <div className="preview-pdf-frame">
          <iframe src={contentUrl} title={file.name} className="preview-iframe" />
        </div>
      );
    case "office":
      return <OfficePreviewPane file={file} contentUrl={contentUrl} />;
    case "text":
      if (textError) return <PreviewMessage message={textError} />;
      if (text === null) {
        return (
          <div className="preview-loading">
            <div className="preview-spinner" aria-hidden />
            <p>Loading text…</p>
          </div>
        );
      }
      if (textEdit) {
        if (textEdit.viewMode === "markdown") {
          const html = renderMarkdownToHtml(textEdit.draft);
          return (
            <div className="preview-markdown-wrap">
              <article className="preview-markdown" dangerouslySetInnerHTML={{ __html: html }} />
            </div>
          );
        }
        if (textEdit.viewMode === "csv-table") {
          const rows = parseCsvPreview(file.name, textEdit.draft);
          return (
            <div className="preview-doc-panel">
              <table className="preview-csv">
                <tbody>
                  {rows.map((row, ri) => (
                    <tr key={ri}>
                      {row.map((cell, ci) => (
                        <td key={ci}>{cell}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
              {rows.length >= 200 ? <p className="preview-footnote">Showing first 200 rows.</p> : null}
              <p className="preview-footnote">Switch to Text to edit. Table view reflects unsaved changes.</p>
            </div>
          );
        }
        if (isSyntaxHighlightFile(file.name)) {
          return (
            <div className="preview-text-wrap preview-text-wrap--edit preview-text-wrap--codemirror">
              <Suspense
                fallback={
                  <textarea
                    className={`preview-text-editor${textWrap ? "" : " preview-text--nowrap"}`}
                    value={textEdit.draft}
                    readOnly
                    aria-label={`Loading editor for ${file.name}`}
                  />
                }
              >
                <CodeMirrorTextEditor
                  fileName={file.name}
                  value={textEdit.draft}
                  textWrap={textWrap}
                  onChange={textEdit.onDraftChange}
                  onSave={textEdit.onSave}
                />
              </Suspense>
            </div>
          );
        }
        return (
          <div className="preview-text-wrap preview-text-wrap--edit">
            <textarea
              className={`preview-text-editor${textWrap ? "" : " preview-text--nowrap"}`}
              value={textEdit.draft}
              onChange={(e) => textEdit.onDraftChange(e.target.value)}
              onKeyDown={(e) => {
                if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
                  e.preventDefault();
                  textEdit.onSave();
                }
              }}
              spellCheck={false}
              aria-label={`Edit ${file.name}`}
            />
          </div>
        );
      }
      if (csvRows && csvRows.length > 0) {
        return (
          <div className="preview-doc-panel">
            <table className="preview-csv">
              <tbody>
                {csvRows.map((row, ri) => (
                  <tr key={ri}>
                    {row.map((cell, ci) => (
                      <td key={ci}>{cell}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
            {csvRows.length >= 200 ? <p className="preview-footnote">Showing first 200 rows.</p> : null}
          </div>
        );
      }
      return (
        <div className="preview-text-wrap">
          {onCopyText ? (
            <div className="preview-text-toolbar">
              <button type="button" className="explorer-btn" onClick={() => onCopyText(text)}>
                Copy all
              </button>
            </div>
          ) : null}
          <HighlightedCodeBlock fileName={file.name} code={text} textWrap={textWrap} />
        </div>
      );
    case "zip":
      if (zipError) return <PreviewMessage message={zipError} />;
      if (!zipEntries) {
        return (
          <div className="preview-loading">
            <div className="preview-spinner" aria-hidden />
            <p>Reading archive…</p>
          </div>
        );
      }
      return (
        <div className="preview-doc-panel">
          <p className="preview-footnote">
            {zipEntries.length} item{zipEntries.length === 1 ? "" : "s"}
            {zipEntries.length >= 2000 ? " (first 2000 shown)" : ""}
          </p>
          <ul className="preview-zip-list">
            {zipEntries.map((e) => (
              <li key={e.path} className={e.dir ? "is-folder" : undefined}>
                {e.dir ? "📁" : "📄"} {e.path}
                {!e.dir && e.size > 0 ? <span className="preview-zip-size"> — {formatBytes(e.size)}</span> : null}
              </li>
            ))}
          </ul>
        </div>
      );
    case "archive":
      return (
        <PreviewMessage
          message="This archive format cannot be browsed in the browser. Download it and open with your desktop app."
        />
      );
    default:
      return (
        <PreviewUnsupported file={file} />
      );
  }
}

function PreviewUnsupported({ file }: { file: CloudFile }) {
  return (
    <div className="preview-unsupported">
      <div className="preview-unsupported-icon" aria-hidden>
        📄
      </div>
      <h3>No preview available</h3>
      <p>Local Cloud cannot render this file type in the browser yet.</p>
      <p className="preview-muted">
        {fileTypeLabel(file)} · {formatBytes(file.size_bytes)}
      </p>
    </div>
  );
}

function PreviewMessage({ message }: { message: string }) {
  return (
    <div className="preview-message">
      <p>{message}</p>
    </div>
  );
}
