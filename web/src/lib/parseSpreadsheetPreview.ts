import * as XLSX from "xlsx";

export type SpreadsheetSheet = {
  name: string;
  rows: string[][];
};

export const PREVIEW_SPREADSHEET_MAX_BYTES = 25 * 1024 * 1024;
export const PREVIEW_SPREADSHEET_MAX_ROWS = 500;
export const PREVIEW_SPREADSHEET_MAX_COLS = 64;

export function parseSpreadsheetPreview(
  data: ArrayBuffer,
  maxRows = PREVIEW_SPREADSHEET_MAX_ROWS,
  maxCols = PREVIEW_SPREADSHEET_MAX_COLS,
): SpreadsheetSheet[] {
  const wb = XLSX.read(data, { type: "array", cellDates: true, dense: true });
  return wb.SheetNames.map((name) => {
    const sheet = wb.Sheets[name];
    if (!sheet) return { name, rows: [] as string[][] };
    const raw = XLSX.utils.sheet_to_json<(string | number | boolean | null)[]>(sheet, {
      header: 1,
      defval: "",
      raw: false,
    });
    const rows = raw.slice(0, maxRows).map((row) =>
      row.slice(0, maxCols).map((cell) => (cell == null ? "" : String(cell))),
    );
    return { name, rows };
  });
}
