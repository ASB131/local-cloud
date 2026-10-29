const OFFICE_EXTENSIONS = new Set([
  "doc",
  "docx",
  "odt",
  "rtf",
  "xls",
  "xlsx",
  "ods",
  "ppt",
  "pptx",
  "odp",
]);

function extOf(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i + 1).toLowerCase();
}

export function isOfficeDocumentFile(name: string, mime: string): boolean {
  const ext = extOf(name);
  if (OFFICE_EXTENSIONS.has(ext)) return true;
  const m = mime.toLowerCase();
  return (
    m.includes("word") ||
    m.includes("excel") ||
    m.includes("spreadsheet") ||
    m.includes("powerpoint") ||
    m.includes("presentation") ||
    m === "application/rtf"
  );
}

const SPREADSHEET_EXTENSIONS = new Set(["xls", "xlsx", "ods", "csv"]);

export function isSpreadsheetOfficeFile(name: string, mime: string): boolean {
  const ext = extOf(name);
  if (SPREADSHEET_EXTENSIONS.has(ext)) return true;
  const m = mime.toLowerCase();
  return m.includes("spreadsheet") || m.includes("excel") || m.includes("csv");
}
