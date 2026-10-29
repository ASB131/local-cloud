import * as XLSX from "xlsx";
import { describe, expect, it } from "vitest";
import { parseSpreadsheetPreview } from "@/lib/parseSpreadsheetPreview";

describe("parseSpreadsheetPreview", () => {
  it("reads sheet names and cell values", () => {
    const wb = XLSX.utils.book_new();
    const ws = XLSX.utils.aoa_to_sheet([
      ["Name", "Qty"],
      ["Apple", 3],
    ]);
    XLSX.utils.book_append_sheet(wb, ws, "Sales");
    const buf = XLSX.write(wb, { type: "array", bookType: "xlsx" }) as ArrayBuffer;
    const sheets = parseSpreadsheetPreview(buf);
    expect(sheets).toHaveLength(1);
    expect(sheets[0].name).toBe("Sales");
    expect(sheets[0].rows[0]).toEqual(["Name", "Qty"]);
    expect(sheets[0].rows[1][0]).toBe("Apple");
  });
});
