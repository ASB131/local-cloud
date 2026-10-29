import { describe, expect, it } from "vitest";
import { highlightFileContent, isSyntaxHighlightFile } from "./highlightCode";

describe("highlightCode", () => {
  it("detects code extensions", () => {
    expect(isSyntaxHighlightFile("app.ts")).toBe(true);
    expect(isSyntaxHighlightFile("notes.txt")).toBe(false);
  });

  it("highlights TypeScript without throwing", () => {
    const { html, language } = highlightFileContent("main.ts", "const x: number = 1;\n");
    expect(language).toBe("typescript");
    expect(html).toContain("number");
  });
});
