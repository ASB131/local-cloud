/** Parse CSV/TSV text into rows for preview (simple split, not RFC 4180 quoted fields). */
export function parseCsvPreview(name: string, text: string, maxRows = 200): string[][] {
  const delim = name.toLowerCase().endsWith(".tsv") ? "\t" : ",";
  const lines = text.split(/\r?\n/).filter((l) => l.length > 0).slice(0, maxRows);
  return lines.map((line) => line.split(delim));
}
