import DOMPurify from "dompurify";
import { Marked } from "marked";
import { highlightFileContent } from "@/lib/highlightCode";

const marked = new Marked({ gfm: true, breaks: true });

marked.use({
  renderer: {
    code({ text, lang }) {
      const language = (lang || "").trim().toLowerCase();
      const pseudoName = language ? `snippet.${language}` : "snippet.txt";
      const { html } = highlightFileContent(pseudoName, text);
      const langClass = language ? ` language-${language}` : "";
      return `<pre class="hljs preview-md-code"><code class="hljs${langClass}">${html}</code></pre>`;
    },
  },
});

const MARKDOWN_ALLOWED_TAGS = [
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "p",
  "br",
  "strong",
  "em",
  "b",
  "i",
  "code",
  "pre",
  "ul",
  "ol",
  "li",
  "blockquote",
  "a",
  "hr",
  "table",
  "thead",
  "tbody",
  "tr",
  "th",
  "td",
  "del",
  "img",
  "span",
];

export function renderMarkdownToHtml(markdown: string): string {
  const raw = marked.parse(markdown) as string;
  return DOMPurify.sanitize(raw, {
    ALLOWED_TAGS: MARKDOWN_ALLOWED_TAGS,
    ALLOWED_ATTR: ["href", "title", "alt", "src", "class"],
  });
}
