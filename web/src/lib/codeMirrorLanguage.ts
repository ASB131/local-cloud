import { cpp } from "@codemirror/lang-cpp";
import { css } from "@codemirror/lang-css";
import { html } from "@codemirror/lang-html";
import { java } from "@codemirror/lang-java";
import { javascript } from "@codemirror/lang-javascript";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { php } from "@codemirror/lang-php";
import { python } from "@codemirror/lang-python";
import { rust } from "@codemirror/lang-rust";
import { sql } from "@codemirror/lang-sql";
import { go } from "@codemirror/lang-go";
import { xml } from "@codemirror/lang-xml";
import { yaml } from "@codemirror/lang-yaml";
import { StreamLanguage } from "@codemirror/language";
import { csharp, kotlin } from "@codemirror/legacy-modes/mode/clike";
import { ruby } from "@codemirror/legacy-modes/mode/ruby";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { swift } from "@codemirror/legacy-modes/mode/swift";
import type { Extension } from "@codemirror/state";

function extOf(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0) return "";
  return name.slice(i + 1).toLowerCase();
}

export function codeMirrorLanguageExtensions(fileName: string): Extension[] {
  const ext = extOf(fileName);
  switch (ext) {
    case "js":
    case "mjs":
    case "cjs":
      return [javascript()];
    case "jsx":
      return [javascript({ jsx: true })];
    case "ts":
      return [javascript({ typescript: true })];
    case "tsx":
      return [javascript({ jsx: true, typescript: true })];
    case "py":
      return [python()];
    case "go":
      return [go()];
    case "rs":
      return [rust()];
    case "java":
      return [java()];
    case "c":
    case "h":
    case "cpp":
    case "hpp":
      return [cpp()];
    case "cs":
      return [StreamLanguage.define(csharp)];
    case "html":
    case "htm":
      return [html()];
    case "css":
      return [css()];
    case "json":
    case "jsonc":
      return [json()];
    case "xml":
      return [xml()];
    case "yaml":
    case "yml":
    case "toml":
      return [yaml()];
    case "sql":
      return [sql()];
    case "sh":
    case "bat":
    case "ps1":
      return [StreamLanguage.define(shell)];
    case "rb":
      return [StreamLanguage.define(ruby)];
    case "php":
      return [php()];
    case "swift":
      return [StreamLanguage.define(swift)];
    case "kt":
      return [StreamLanguage.define(kotlin)];
    case "md":
    case "markdown":
      return [markdown()];
    default:
      return [];
  }
}
