import { useEffect, useState } from "react";
import { issueMediaToken } from "@/api/files";
import type { CloudFile } from "@/api/types";
import { fileIconUrl } from "@/lib/fileIconAsset";
import { getMediaToken, thumbnailUrl } from "@/media/previewUrl";

type Props = {
  file: CloudFile;
  variant?: "grid" | "inline";
};

export function ExplorerFileIcon({ file, variant = "grid" }: Props) {
  const [src, setSrc] = useState<string | null>(null);

  useEffect(() => {
    if (file.thumbnail_status !== "ready") {
      setSrc(null);
      return;
    }
    let cancelled = false;
    getMediaToken(file.id, issueMediaToken)
      .then((token) => {
        if (!cancelled) setSrc(thumbnailUrl(file.id, token));
      })
      .catch(() => {
        if (!cancelled) setSrc(null);
      });
    return () => {
      cancelled = true;
    };
  }, [file.id, file.thumbnail_status]);

  if (src) {
    return (
      <img
        src={src}
        alt=""
        className={variant === "grid" ? "explorer-tile-thumb" : "explorer-inline-thumb"}
        draggable={false}
      />
    );
  }

  const shellClass =
    variant === "grid"
      ? "explorer-file-type-icon-shell explorer-tile-icon"
      : "explorer-file-type-icon-shell explorer-file-type-icon-shell--inline";

  return (
    <span className={shellClass}>
      <img src={fileIconUrl(file)} alt="" className="explorer-file-type-icon" draggable={false} />
    </span>
  );
}
