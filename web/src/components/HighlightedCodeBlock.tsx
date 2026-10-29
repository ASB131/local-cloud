import { useMemo } from "react";
import { highlightFileContent, isSyntaxHighlightFile } from "@/lib/highlightCode";

type Props = {
  fileName: string;
  code: string;
  textWrap: boolean;
};

export function HighlightedCodeBlock({ fileName, code, textWrap }: Props) {
  const highlighted = useMemo(() => {
    if (!isSyntaxHighlightFile(fileName)) return null;
    return highlightFileContent(fileName, code);
  }, [fileName, code]);

  if (!highlighted) {
    return <pre className={`preview-text${textWrap ? "" : " preview-text--nowrap"}`}>{code}</pre>;
  }

  return (
    <pre className={`preview-text preview-code${textWrap ? "" : " preview-text--nowrap"}`}>
      <code
        className={`hljs${highlighted.language ? ` language-${highlighted.language}` : ""}`}
        dangerouslySetInnerHTML={{ __html: highlighted.html }}
      />
    </pre>
  );
}
