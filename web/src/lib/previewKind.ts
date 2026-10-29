import type { CloudFile } from "@/api/types";
import { isOfficeDocumentFile } from "@/lib/officePreview";

export type PreviewKind =
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "text"
  | "office"
  | "zip"
  | "archive"
  | "unsupported";

export const PREVIEW_TEXT_MAX_BYTES = 5 * 1024 * 1024;
export const PREVIEW_ZIP_MAX_BYTES = 200 * 1024 * 1024;

const TEXT_EXTENSIONS = new Set([
  "txt",
  "md",
  "markdown",
  "json",
  "jsonc",
  "csv",
  "tsv",
  "xml",
  "yaml",
  "yml",
  "log",
  "ini",
  "cfg",
  "conf",
  "html",
  "htm",
  "css",
  "js",
  "mjs",
  "cjs",
  "ts",
  "tsx",
  "jsx",
  "py",
  "go",
  "rs",
  "java",
  "c",
  "cpp",
  "h",
  "hpp",
  "cs",
  "sh",
  "bat",
  "ps1",
  "sql",
  "env",
  "toml",
]);

const IMAGE_EXT = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico", "avif"]);
const VIDEO_EXT = new Set(["mp4", "webm", "ogg", "mov", "mkv", "m4v"]);
const AUDIO_EXT = new Set(["mp3", "wav", "ogg", "flac", "m4a", "aac", "opus"]);
const ARCHIVE_EXT = new Set(["rar", "7z", "tar", "gz", "bz2", "xz", "tgz"]);

function extOf(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i + 1).toLowerCase();
}

export function previewKind(file: Pick<CloudFile, "name" | "mime_type">): PreviewKind {
  const mime = (file.mime_type || "").toLowerCase();
  const ext = extOf(file.name);

  if (mime.startsWith("image/") || IMAGE_EXT.has(ext)) return "image";
  if (mime.startsWith("video/") || VIDEO_EXT.has(ext)) return "video";
  if (mime.startsWith("audio/") || AUDIO_EXT.has(ext)) return "audio";
  if (mime === "application/pdf" || ext === "pdf") return "pdf";
  if (isOfficeDocumentFile(file.name, mime)) return "office";

  if (
    mime.startsWith("text/") ||
    mime === "application/json" ||
    mime === "application/xml" ||
    mime === "application/javascript" ||
    mime.includes("markdown") ||
    TEXT_EXTENSIONS.has(ext)
  ) {
    return "text";
  }

  if (mime.includes("zip") || ext === "zip") return "zip";
  if (mime.includes("rar") || mime.includes("x-7z") || ARCHIVE_EXT.has(ext)) return "archive";

  return "unsupported";
}

export function formatTextPreview(name: string, raw: string): string {
  const ext = extOf(name);
  if (ext === "json" || ext === "jsonc") {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }
  return raw;
}

export function isCsvFile(name: string, mime: string): boolean {
  const ext = extOf(name);
  return ext === "csv" || ext === "tsv" || mime.includes("csv");
}

export function isMarkdownFile(name: string, mime: string): boolean {
  const ext = extOf(name);
  return ext === "md" || ext === "markdown" || mime.includes("markdown");
}

export function isEditableTextFile(file: Pick<CloudFile, "name" | "mime_type" | "size_bytes">): boolean {
  if (previewKind(file) !== "text") return false;
  return file.size_bytes <= PREVIEW_TEXT_MAX_BYTES;
}

export function textEditBaseline(name: string, raw: string): string {
  return formatTextPreview(name, raw);
}
