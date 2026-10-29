import { useEffect, useMemo, useState } from "react";
import type { CloudFile } from "@/api/types";
import {
  PREVIEW_SPREADSHEET_MAX_BYTES,
  PREVIEW_SPREADSHEET_MAX_COLS,
  PREVIEW_SPREADSHEET_MAX_ROWS,
  parseSpreadsheetPreview,
} from "@/lib/parseSpreadsheetPreview";

type Props = {
  file: CloudFile;
  contentUrl: string;
};

function colLabel(index: number): string {
  let n = index;
  let label = "";
  do {
    label = String.fromCharCode(65 + (n % 26)) + label;
    n = Math.floor(n / 26) - 1;
  } while (n >= 0);
  return label;
}

export function SpreadsheetPreviewPane({ file, contentUrl }: Props) {
  const [sheets, setSheets] = useState<ReturnType<typeof parseSpreadsheetPreview> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    setSheets(null);
    setError(null);
    setActiveIndex(0);
    if (file.size_bytes > PREVIEW_SPREADSHEET_MAX_BYTES) {
      setError(
        `Spreadsheet is too large to preview in the browser (${Math.round(file.size_bytes / (1024 * 1024))} MiB). Download to open in Excel.`,
      );
      return;
    }
    let cancelled = false;
    fetch(contentUrl)
      .then((r) => {
        if (!r.ok) throw new Error("fetch failed");
        return r.arrayBuffer();
      })
      .then((buf) => {
        if (cancelled) return;
        setSheets(parseSpreadsheetPreview(buf));
      })
      .catch(() => {
        if (!cancelled) setError("Could not load spreadsheet preview.");
      });
    return () => {
      cancelled = true;
    };
  }, [contentUrl, file.size_bytes]);

  const active = sheets?.[activeIndex];
  const colCount = useMemo(() => {
    if (!active?.rows.length) return 0;
    return Math.min(
      PREVIEW_SPREADSHEET_MAX_COLS,
      active.rows.reduce((max, row) => Math.max(max, row.length), 0),
    );
  }, [active]);

  if (error) {
    return (
      <div className="preview-message">
        <p>{error}</p>
      </div>
    );
  }
  if (!sheets) {
    return (
      <div className="preview-loading">
        <div className="preview-spinner" aria-hidden />
        <p>Loading spreadsheet…</p>
      </div>
    );
  }
  if (!active || active.rows.length === 0) {
    return (
      <div className="preview-message">
        <p>This workbook has no data to display.</p>
      </div>
    );
  }

  const truncatedRows = active.rows.length >= PREVIEW_SPREADSHEET_MAX_ROWS;
  const truncatedCols = colCount >= PREVIEW_SPREADSHEET_MAX_COLS;

  return (
    <div className="preview-spreadsheet-shell">
      <div className="preview-spreadsheet-grid-wrap">
        <table className="preview-spreadsheet">
          <thead>
            <tr>
              <th className="preview-spreadsheet-corner" scope="col" aria-hidden />
              {Array.from({ length: colCount }, (_, ci) => (
                <th key={ci} scope="col" className="preview-spreadsheet-col-head">
                  {colLabel(ci)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {active.rows.map((row, ri) => (
              <tr key={ri}>
                <th scope="row" className="preview-spreadsheet-row-head">
                  {ri + 1}
                </th>
                {Array.from({ length: colCount }, (_, ci) => (
                  <td key={ci} title={row[ci] ?? ""}>
                    {row[ci] ?? ""}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {truncatedRows || truncatedCols ? (
        <p className="preview-spreadsheet-footnote">
          Showing up to {PREVIEW_SPREADSHEET_MAX_ROWS} rows × {PREVIEW_SPREADSHEET_MAX_COLS} columns.
        </p>
      ) : null}
      {sheets.length > 1 ? (
        <div className="preview-spreadsheet-tabs" role="tablist" aria-label="Worksheets">
          {sheets.map((sheet, i) => (
            <button
              key={sheet.name}
              type="button"
              role="tab"
              aria-selected={i === activeIndex}
              className={`preview-spreadsheet-tab${i === activeIndex ? " is-active" : ""}`}
              onClick={() => setActiveIndex(i)}
            >
              {sheet.name}
            </button>
          ))}
        </div>
      ) : (
        <div className="preview-spreadsheet-tabs preview-spreadsheet-tabs--single">
          <span className="preview-spreadsheet-tab is-active">{active.name}</span>
        </div>
      )}
    </div>
  );
}
