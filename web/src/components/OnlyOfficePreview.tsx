import { useQuery } from "@tanstack/react-query";
import { useEffect, useId, useRef } from "react";
import { getOnlyOfficeConfig } from "@/api/files";

type DocsConfig = {
  document: Record<string, unknown>;
  documentType: string;
  editorConfig: Record<string, unknown>;
  height: string;
  width: string;
};

declare global {
  interface Window {
    DocsAPI?: { DocEditor: new (id: string, cfg: DocsConfig) => unknown };
  }
}

type Props = {
  fileId: string;
};

export function OnlyOfficePreview({ fileId }: Props) {
  const mountId = useId().replace(/:/g, "");
  const hostRef = useRef<HTMLDivElement>(null);
  const config = useQuery({
    queryKey: ["onlyoffice", fileId],
    queryFn: () => getOnlyOfficeConfig(fileId),
    retry: false,
  });

  useEffect(() => {
    if (!config.data || !hostRef.current) return;
    const base = config.data.document_server_url.replace(/\/$/, "");
    let editor: unknown;
    let script: HTMLScriptElement | null = document.querySelector(`script[data-onlyoffice="${base}"]`);
    const mount = () => {
      if (!window.DocsAPI || !hostRef.current) return;
      editor = new window.DocsAPI.DocEditor(mountId, {
        document: config.data!.document,
        documentType: config.data!.document_type,
        editorConfig: { mode: "view", lang: "en", customization: { compactHeader: true } },
        height: "100%",
        width: "100%",
      });
    };
    if (window.DocsAPI) {
      mount();
      return () => {
        editor = null;
      };
    }
    if (!script) {
      script = document.createElement("script");
      script.src = `${base}/web-apps/apps/api/documents/api.js`;
      script.async = true;
      script.dataset.onlyoffice = base;
      document.body.appendChild(script);
    }
    script.addEventListener("load", mount);
    return () => {
      script?.removeEventListener("load", mount);
      editor = null;
    };
  }, [config.data, mountId]);

  if (config.isLoading) {
    return (
      <div className="preview-loading">
        <div className="preview-spinner" aria-hidden />
        <p>Loading document viewer…</p>
      </div>
    );
  }
  if (config.isError) {
    return (
      <div className="preview-message">
        <p>OnlyOffice preview is not available. Use the LibreOffice PDF preview or download the file.</p>
      </div>
    );
  }
  return <div id={mountId} ref={hostRef} className="preview-onlyoffice" />;
}
