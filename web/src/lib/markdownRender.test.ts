/** @vitest-environment happy-dom */
import { Marked } from "marked";
import { describe, expect, it } from "vitest";
import { renderMarkdownToHtml } from "./markdownRender";

describe("renderMarkdownToHtml", () => {
  it("marked parses headings", () => {
    const m = new Marked({ gfm: true, breaks: true });
    expect(String(m.parse("# Title"))).toContain("<h1");
  });

  it("renders emphasis through pipeline", () => {
    const html = renderMarkdownToHtml("**bold**");
    expect(html).toContain("bold");
    expect(html.toLowerCase()).toContain("strong");
  });

  it("strips dangerous attributes from raw HTML", () => {
    const html = renderMarkdownToHtml('<img src=x onerror="alert(1)">');
    expect(html.toLowerCase()).not.toContain("onerror");
  });
});
