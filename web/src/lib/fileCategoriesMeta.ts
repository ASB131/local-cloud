export const FILE_CATEGORY_LABELS: Record<string, string> = {
  images: "Images",
  videos: "Videos",
  audio: "Audio",
  pdfs: "PDF documents",
  archives: "Compressed files",
  documents: "Documents",
  spreadsheets: "Spreadsheets",
  presentations: "Presentations",
  code: "Code & markup",
};

export function fileCategoryLabel(id: string): string {
  return FILE_CATEGORY_LABELS[id] ?? id;
}
