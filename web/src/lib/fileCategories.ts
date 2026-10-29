export const FILE_CATEGORY_ICONS: Record<string, string> = {
  images: "🖼️",
  videos: "🎬",
  audio: "🎵",
  pdfs: "📕",
  archives: "📦",
  documents: "📘",
  spreadsheets: "📊",
  presentations: "📽️",
  code: "💻",
};

export function fileCategoryIcon(id: string): string {
  return FILE_CATEGORY_ICONS[id] ?? "📁";
}
