import type { CloudFile } from "@/api/types";

export type OfficeIconVariant = "word" | "excel" | "powerpoint" | null;

function extLower(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i + 1).toLowerCase();
}

export function officeIconVariant(file: Pick<CloudFile, "name" | "mime_type">): OfficeIconVariant {
  const mime = (file.mime_type || "").toLowerCase();
  const ext = extLower(file.name);
  if (["doc", "docx", "odt", "rtf"].includes(ext) || mime.includes("word") || mime.includes("opendocument.text")) {
    return "word";
  }
  if (["xls", "xlsx", "ods", "csv"].includes(ext) || mime.includes("spreadsheet") || mime.includes("excel")) {
    return "excel";
  }
  if (["ppt", "pptx", "odp"].includes(ext) || mime.includes("presentation") || mime.includes("powerpoint")) {
    return "powerpoint";
  }
  return null;
}

export function fileIconEmoji(file: Pick<CloudFile, "name" | "mime_type">): string {
  const mime = (file.mime_type || "").toLowerCase();
  const ext = extLower(file.name);
  const office = officeIconVariant(file);
  if (office === "word") return "W";
  if (office === "excel") return "X";
  if (office === "powerpoint") return "P";

  if (mime.startsWith("image/")) return "🖼️";
  if (mime.startsWith("video/")) return "🎬";
  if (mime.startsWith("audio/")) return "🎵";
  if (mime === "application/pdf" || ext === "pdf") return "📕";
  if (
    mime.includes("zip") ||
    mime.includes("archive") ||
    ["zip", "rar", "7z", "tar", "gz", "bz2"].includes(ext)
  ) {
    return "📦";
  }
  if (
    mime.startsWith("text/") ||
    ["txt", "md", "json", "xml", "yaml", "yml", "csv", "log"].includes(ext)
  ) {
    return "📝";
  }
  if (["js", "ts", "tsx", "jsx", "py", "go", "rs", "java", "c", "cpp", "cs", "html", "css"].includes(ext)) {
    return "💻";
  }
  return "📄";
}

export function fileTypeLabel(file: Pick<CloudFile, "name" | "mime_type">): string {
  const mime = (file.mime_type || "").toLowerCase();
  const ext = file.name.includes(".") ? file.name.split(".").pop()!.toUpperCase() : "";

  if (mime.startsWith("image/")) {
    const sub = mime.slice("image/".length).toUpperCase();
    return sub ? `${sub} image` : "Image";
  }
  if (mime.startsWith("video/")) return "Video";
  if (mime.startsWith("audio/")) return "Audio";
  if (mime === "application/pdf" || ext === "PDF") return "PDF document";
  if (mime.includes("zip") || ext === "ZIP") return "Compressed folder";
  if (ext === "PY") return "Python File";
  if (ext === "TXT") return "Text Document";
  if (ext === "HTML" || ext === "HTM") return "HTML Document";
  if (mime.startsWith("text/")) return "Text document";
  if (mime.includes("word") || ["DOC", "DOCX"].includes(ext)) return "Document";
  if (mime.includes("spreadsheet") || ["XLS", "XLSX", "CSV"].includes(ext)) return "Spreadsheet";
  if (mime.includes("presentation") || ["PPT", "PPTX"].includes(ext)) return "Presentation";
  if (ext) return `${ext} file`;
  if (mime) return mime;
  return "File";
}

export const FOLDER_TYPE_LABEL = "File folder";
