import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { searchFiles } from "@/api/files";
import type { CloudFile, Folder } from "@/api/types";
import { ExplorerFileIcon } from "@/components/ExplorerFileIcon";
import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";
import { formatBytes } from "@/lib/formatBytes";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { fileTypeLabel } from "@/lib/fileIcon";

type SearchFile = CloudFile & { folder_name?: string };

export function CloudSearchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const qParam = searchParams.get("q") ?? "";
  const [draft, setDraft] = useState(qParam);
  const [debouncedQ, setDebouncedQ] = useState(qParam.trim());
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  useEffect(() => {
    setDraft(qParam);
    setDebouncedQ(qParam.trim());
  }, [qParam]);

  useEffect(() => {
    const t = window.setTimeout(() => setDebouncedQ(draft.trim()), 300);
    return () => window.clearTimeout(t);
  }, [draft]);

  const search = useQuery({
    queryKey: ["search", debouncedQ],
    queryFn: () => searchFiles(debouncedQ),
    enabled: debouncedQ.length >= 1,
  });

  const { folders, files } = useMemo(() => {
    const hits = search.data ?? [];
    const folders: Folder[] = [];
    const files: SearchFile[] = [];
    for (const hit of hits) {
      if (hit.kind === "folder") folders.push(hit.item as Folder);
      else files.push(hit.item as SearchFile);
    }
    return { folders, files };
  }, [search.data]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = draft.trim();
    if (!q) {
      setSearchParams({}, { replace: true });
      return;
    }
    setSearchParams({ q }, { replace: true });
  };

  const openFile = (f: SearchFile) => {
    navigate(`/cloud/folder/${f.folder_id}?preview=${encodeURIComponent(f.id)}`);
  };

  return (
    <>
      <div className="explorer-titlebar explorer-titlebar--home">
        <form className="explorer-home-search-form explorer-search-form-wide" onSubmit={submit}>
          <input
            type="search"
            className="explorer-home-search"
            placeholder="Search all files and folders…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            autoFocus
            aria-label="Search"
          />
          <button type="submit" className="explorer-btn explorer-btn-primary">
            Search
          </button>
        </form>
      </div>
      <div className="explorer-content explorer-search-page">
        {!debouncedQ ? (
          <p className="explorer-empty-hint">Type a name to search your entire cloud. Use several words to narrow results.</p>
        ) : null}
        {debouncedQ && search.isLoading ? <p className="text-[#9a9a9a]">Searching…</p> : null}
        {debouncedQ && !search.isLoading && folders.length === 0 && files.length === 0 ? (
          <p className="explorer-empty-hint">No matches for &ldquo;{debouncedQ}&rdquo;</p>
        ) : null}

        {folders.length > 0 ? (
          <section className="explorer-search-section">
            <h2 className="explorer-home-card-title">Folders ({folders.length})</h2>
            <ul className="explorer-search-results">
              {folders.map((f) => (
                <li key={f.id}>
                  <button
                    type="button"
                    className={`explorer-search-row${selectedKey === `folder:${f.id}` ? " selected" : ""}`}
                    onClick={() => setSelectedKey(`folder:${f.id}`)}
                    onDoubleClick={() => navigate(`/cloud/folder/${f.id}`)}
                  >
                    <span className="explorer-search-row-icon" aria-hidden>
                      <ExplorerFolderIcon variant="inline" />
                    </span>
                    <span className="explorer-search-row-name" title={f.name}>
                      {fileDisplayName(f.name)}
                    </span>
                    <span className="explorer-search-row-meta">Folder</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ) : null}

        {files.length > 0 ? (
          <section className="explorer-search-section">
            <h2 className="explorer-home-card-title">Files ({files.length})</h2>
            <ul className="explorer-search-results">
              {files.map((f) => (
                <li key={f.id}>
                  <button
                    type="button"
                    className={`explorer-search-row${selectedKey === `file:${f.id}` ? " selected" : ""}`}
                    onClick={() => setSelectedKey(`file:${f.id}`)}
                    onDoubleClick={() => openFile(f)}
                  >
                    <span className="explorer-search-row-icon">
                      <ExplorerFileIcon file={f} variant="inline" />
                    </span>
                    <span className="explorer-search-row-name" title={f.name}>
                      {fileDisplayName(f.name)}
                    </span>
                    <span className="explorer-search-row-meta">{f.folder_name || "My files"}</span>
                    <span className="explorer-search-row-meta">{fileTypeLabel(f)}</span>
                    <span className="explorer-search-row-meta tabular-nums">{formatBytes(f.size_bytes)}</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ) : null}
      </div>
      <div className="explorer-statusbar">
        <span>
          {debouncedQ
            ? search.isLoading
              ? "Searching…"
              : `${folders.length + files.length} result${folders.length + files.length === 1 ? "" : "s"}`
            : "Search"}
        </span>
      </div>
    </>
  );
}
