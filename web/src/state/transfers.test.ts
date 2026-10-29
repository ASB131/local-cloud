import { describe, expect, it } from "vitest";
import { transferReducer, type TransferItem } from "./transfers";

const base: TransferItem = {
  id: "1",
  kind: "upload",
  name: "a.txt",
  totalBytes: 100,
  doneBytes: 0,
  status: "queued",
};

describe("transferReducer", () => {
  it("adds and patches progress", () => {
    let s = transferReducer([], { type: "add", item: base });
    s = transferReducer(s, { type: "patch", id: "1", patch: { doneBytes: 50, status: "running" } });
    expect(s[0]?.doneBytes).toBe(50);
    expect(s[0]?.status).toBe("running");
  });

  it("marks cancel without removing", () => {
    const s = transferReducer([{ ...base, status: "running" }], { type: "cancel", id: "1" });
    expect(s[0]?.status).toBe("cancelled");
  });
});
