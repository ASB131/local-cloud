import type { CloudFile } from "@/api/types";
import { officeIconVariant } from "@/lib/fileIcon";

/** Basename under `/icons/file-types/`. */
export type FileIconAsset =
  | "word"
  | "doc"
  | "docx"
  | "excel"
  | "xls"
  | "xlsx"
  | "powerpoint"
  | "ppt"
  | "pptx"
  | "image"
  | "png"
  | "gif"
  | "video"
  | "mp4"
  | "audio"
  | "mp3"
  | "pdf"
  | "zip"
  | "rar"
  | "archive"
  | "text"
  | "html"
  | "css"
  | "csv"
  | "xml"
  | "log"
  | "js"
  | "code"
  | "exe"
  | "rtf"
  | "cdr"
  | "file";

const ICON_BASE = "/icons/file-types";

const CODE_EXTENSIONS = new Set([
  "ts",
  "tsx",
  "jsx",
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
]);

const TEXT_EXTENSIONS = new Set([
  "txt",
  "md",
  "markdown",
  "json",
  "jsonc",
  "yaml",
  "yml",
  "ini",
  "cfg",
  "conf",
]);

function extLower(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i + 1).toLowerCase();
}

export function fileIconAsset(file: Pick<CloudFile, "name" | "mime_type">): FileIconAsset {
  const mime = (file.mime_type || "").toLowerCase();
  const ext = extLower(file.name);

  if (ext === "py" || mime.includes("python")) return "code";
  if (ext === "js" || ext === "mjs" || ext === "cjs") return "js";
  if (ext === "mp3") return "mp3";
  if (ext === "mp4") return "mp4";
  if (ext === "exe" || ext === "msi") return "exe";
  if (ext === "rtf") return "rtf";
  if (ext === "cdr") return "cdr";

  const office = officeIconVariant(file);
  if (office === "word") {
    if (ext === "doc") return "doc";
    if (ext === "docx") return "docx";
    return "word";
  }
  if (office === "excel") {
    if (ext === "csv") return "csv";
    if (ext === "xls") return "xls";
    if (ext === "xlsx") return "xlsx";
    return "excel";
  }
  if (office === "powerpoint") {
    if (ext === "ppt") return "ppt";
    if (ext === "pptx") return "pptx";
    return "powerpoint";
  }

  if (ext === "gif") return "gif";
  if (ext === "png") return "png";
  if (ext === "html" || ext === "htm") return "html";
  if (ext === "css") return "css";
  if (ext === "csv" || ext === "tsv") return "csv";
  if (ext === "xml") return "xml";
  if (ext === "log") return "log";
  if (ext === "zip") return "zip";
  if (ext === "rar") return "rar";
  if (["7z", "tar", "gz", "bz2", "tgz"].includes(ext)) return "archive";

  if (mime.startsWith("image/")) return "image";
  if (mime.startsWith("video/")) return "video";
  if (mime.startsWith("audio/")) return "audio";
  if (mime === "application/pdf" || ext === "pdf") return "pdf";

  if (mime.includes("zip") || mime.includes("archive") || mime.includes("compressed")) {
    return "zip";
  }

  if (CODE_EXTENSIONS.has(ext)) return "code";

  if (mime.startsWith("text/") || TEXT_EXTENSIONS.has(ext)) return "text";

  return "file";
}

export function fileIconUrl(file: Pick<CloudFile, "name" | "mime_type">): string {
  return `${ICON_BASE}/${fileIconAsset(file)}.png`;
}
