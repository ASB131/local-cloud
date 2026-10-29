import { useEffect, useState } from "react";
import { NavLink, useParams } from "react-router-dom";
import { useAuth } from "@/state/auth";
import { loadRecentFolders, type RecentFolder } from "@/state/explorerRecent";

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `explorer-nav-item explorer-nav-item--compact${isActive ? " active" : ""}`;

export function ExplorerRecentNav() {
  const { user } = useAuth();
  const { folderId: paramId } = useParams();
  const rootFolderId = user?.root_folder_id ?? "";
  const currentId = paramId ?? rootFolderId;
  const [recent, setRecent] = useState<RecentFolder[]>(() => loadRecentFolders());

  useEffect(() => {
    const refresh = () => setRecent(loadRecentFolders());
    refresh();
    window.addEventListener("storage", refresh);
    const id = window.setInterval(refresh, 2000);
    return () => {
      window.removeEventListener("storage", refresh);
      window.clearInterval(id);
    };
  }, [currentId]);

  if (recent.length === 0) return null;

  return (
    <div className="explorer-nav-recent">
      <div className="explorer-nav-section-label">Recent</div>
      {recent.map((f) => (
        <NavLink
          key={f.id}
          to={`/cloud/folder/${f.id}`}
          className={linkClass}
          title={f.name}
        >
          <span className="explorer-nav-recent-icon" aria-hidden>
            📂
          </span>
          <span className="explorer-nav-recent-name">{f.name}</span>
        </NavLink>
      ))}
    </div>
  );
}
