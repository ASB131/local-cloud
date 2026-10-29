import { oneDark } from "@codemirror/theme-one-dark";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { highlightSelectionMatches, searchKeymap } from "@codemirror/search";
import { EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, drawSelection, highlightActiveLine, highlightSpecialChars } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import { useMemo, useRef } from "react";
import { codeMirrorLanguageExtensions } from "@/lib/codeMirrorLanguage";
import { isSyntaxHighlightFile } from "@/lib/highlightCode";

type Props = {
  fileName: string;
  value: string;
  textWrap: boolean;
  onChange: (value: string) => void;
  onSave: () => void;
};

const editorTheme = EditorView.theme(
  {
    "&": {
      height: "auto",
      backgroundColor: "#121212",
    },
    ".cm-scroller": {
      fontFamily: 'ui-monospace, Consolas, "Cascadia Mono", monospace',
      lineHeight: "1.55",
      overflow: "visible",
    },
    ".cm-content": {
      minHeight: "240px",
    },
    "&.cm-focused": {
      outline: "none",
    },
    ".cm-gutters": {
      backgroundColor: "#0f0f0f",
      color: "#6b6b6b",
      border: "none",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "#1a1a1a",
    },
    ".cm-activeLine": {
      backgroundColor: "rgba(255, 255, 255, 0.04)",
    },
  },
  { dark: true },
);

function useEditorExtensions(fileName: string, textWrap: boolean, onSave: () => void): Extension[] {
  const onSaveRef = useRef(onSave);
  onSaveRef.current = onSave;

  return useMemo(() => {
    const saveKeymap = keymap.of([
      {
        key: "Mod-s",
        preventDefault: true,
        run: () => {
          onSaveRef.current();
          return true;
        },
      },
    ]);

    const lang = isSyntaxHighlightFile(fileName) ? codeMirrorLanguageExtensions(fileName) : [];

    const wrap = textWrap ? EditorView.lineWrapping : [];

    return [
      lineNumbers(),
      highlightSpecialChars(),
      history(),
      drawSelection(),
      highlightActiveLine(),
      highlightSelectionMatches(),
      EditorView.contentAttributes.of({ "aria-label": `Edit ${fileName}` }),
      EditorState.allowMultipleSelections.of(true),
      keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
      Prec.highest(saveKeymap),
      oneDark,
      editorTheme,
      ...lang,
      wrap,
    ];
  }, [fileName, textWrap]);
}

export function CodeMirrorTextEditor({ fileName, value, textWrap, onChange, onSave }: Props) {
  const extensions = useEditorExtensions(fileName, textWrap, onSave);

  return (
    <div className="preview-code-editor">
      <CodeMirror
        value={value}
        theme="none"
        extensions={extensions}
        onChange={onChange}
        basicSetup={false}
      />
    </div>
  );
}
