import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { ExplorerRecentNav } from "@/components/ExplorerRecentNav";
import { ExplorerNavProvider } from "@/state/explorerNav";
import { useAuth } from "@/state/auth";

const navLink = ({ isActive }: { isActive: boolean }) =>
  `explorer-nav-item${isActive ? " active" : ""}`;

export function CloudLayout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  return (
    <div className="explorer-shell">
      <aside className="explorer-nav">
        <div className="px-3 pb-2 text-[11px] font-semibold uppercase tracking-wide text-[#6e6e6e]">
          Local Cloud
        </div>
        <NavLink to="/cloud" end className={navLink}>
          <span aria-hidden>🏠</span> Home
        </NavLink>
        {user?.root_folder_id ? (
          <NavLink to={`/cloud/folder/${user.root_folder_id}`} className={navLink}>
            <span aria-hidden>📂</span> Files
          </NavLink>
        ) : null}
        <NavLink to="/cloud/favorites" className={navLink}>
          <span aria-hidden>⭐</span> Favorites
        </NavLink>
        <NavLink to="/cloud/trash" className={navLink}>
          <span aria-hidden>🗑️</span> Recycle Bin
        </NavLink>
        <ExplorerRecentNav />
        <div className="my-2 border-t border-[#3d3d3d]" />
        {user?.is_admin ? (
          <button type="button" className="explorer-nav-item w-[calc(100%-16px)] border-0 bg-transparent text-left" onClick={() => navigate("/admin")}>
            <span aria-hidden>⚙️</span> Admin
          </button>
        ) : null}
        <button type="button" className="explorer-nav-item mt-auto w-[calc(100%-16px)] border-0 bg-transparent text-left" onClick={() => navigate("/settings")}>
          <span aria-hidden>👤</span> {user?.username}
        </button>
        <button
          type="button"
          className="explorer-nav-item w-[calc(100%-16px)] border-0 bg-transparent text-left text-[#9a9a9a]"
          onClick={() => void logout()}
        >
          Sign out
        </button>
      </aside>
      <div className="explorer-main">
        <ExplorerNavProvider>
          <Outlet />
        </ExplorerNavProvider>
      </div>
    </div>
  );
}
