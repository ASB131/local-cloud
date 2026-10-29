import { Link } from "react-router-dom";
import { useAuth } from "@/state/auth";

export function SettingsPage() {
  const { user, logout } = useAuth();
  return (
    <div className="mx-auto max-w-lg px-4 py-8">
      <Link to="/cloud" className="text-sm text-emerald-400 hover:underline">
        ← Back to cloud
      </Link>
      <h1 className="mt-4 text-2xl font-semibold">Settings</h1>
      <dl className="mt-6 space-y-3 text-sm">
        <div>
          <dt className="text-zinc-500">Username</dt>
          <dd>{user?.username}</dd>
        </div>
        <div>
          <dt className="text-zinc-500">Storage used</dt>
          <dd>
            {user ? Math.round(user.storage_used_bytes / (1024 * 1024)) : 0} MiB /{" "}
            {user?.storage_quota_bytes
              ? Math.round(user.storage_quota_bytes / (1024 * 1024))
              : 0}{" "}
            MiB
          </dd>
        </div>
      </dl>
      <button
        type="button"
        className="mt-8 rounded-lg bg-zinc-800 px-4 py-2 text-sm hover:bg-zinc-700"
        onClick={() => void logout()}
      >
        Sign out
      </button>
    </div>
  );
}
