import { useQuery } from "@tanstack/react-query";
import { Suspense, lazy, useEffect, useState } from "react";
import { getClientFeatures, issueMediaToken } from "@/api/files";
import type { CloudFile } from "@/api/types";
import { OnlyOfficePreview } from "@/components/OnlyOfficePreview";
import { isSpreadsheetOfficeFile } from "@/lib/officePreview";
import { getMediaToken, officePreviewUrl } from "@/media/previewUrl";

const SpreadsheetPreviewPane = lazy(() =>
  import("@/components/SpreadsheetPreviewPane").then((m) => ({ default: m.SpreadsheetPreviewPane })),
);

type Props = {
  file: CloudFile;
  contentUrl: string | null;
};

function PreviewMessage({ message }: { message: string }) {
  return (
    <div className="preview-message">
      <p>{message}</p>
    </div>
  );
}

export function OfficePreviewPane({ file, contentUrl }: Props) {
  const features = useQuery({
    queryKey: ["client-features"],
    queryFn: getClientFeatures,
    staleTime: 300_000,
  });
  const [pdfSrc, setPdfSrc] = useState<string | null>(null);
  const status = file.office_preview_status || "none";
  const ooUrl = features.data?.onlyoffice_public_url?.replace(/\/$/, "") ?? "";
  const useOnlyOffice =
    Boolean(ooUrl) &&
    ooUrl !== window.location.origin &&
    !ooUrl.startsWith(`${window.location.protocol}//${window.location.host}`);

  useEffect(() => {
    if (useOnlyOffice) return;
    if (status !== "ready") {
      setPdfSrc(null);
      return;
    }
    let cancelled = false;
    (async () => {
      const token = await getMediaToken(file.id, issueMediaToken);
      if (!cancelled) setPdfSrc(officePreviewUrl(file.id, token));
    })().catch(() => {
      if (!cancelled) setPdfSrc(null);
    });
    return () => {
      cancelled = true;
    };
  }, [file.id, status, useOnlyOffice]);

  if (useOnlyOffice) {
    return <OnlyOfficePreview fileId={file.id} />;
  }
  if (isSpreadsheetOfficeFile(file.name, file.mime_type) && contentUrl) {
    return (
      <Suspense
        fallback={
          <div className="preview-loading">
            <div className="preview-spinner" aria-hidden />
            <p>Loading spreadsheet…</p>
          </div>
        }
      >
        <SpreadsheetPreviewPane file={file} contentUrl={contentUrl} />
      </Suspense>
    );
  }
  if (status === "pending" || status === "none") {
    return (
      <div className="preview-loading">
        <div className="preview-spinner" aria-hidden />
        <p>Preparing document preview…</p>
        <p className="preview-footnote">LibreOffice is converting this file to PDF on the server.</p>
      </div>
    );
  }
  if (status === "ready" && pdfSrc) {
    return (
      <div className="preview-pdf-frame">
        <iframe src={pdfSrc} title={`${file.name} (preview)`} className="preview-iframe" />
      </div>
    );
  }
  return (
    <PreviewMessage message="Document preview failed or is not ready. Download the file to open it on your PC." />
  );
}
