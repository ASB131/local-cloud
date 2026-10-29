import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, Navigate } from "react-router-dom";
import {
  createInvite,
  deleteUser,
  getLastRebuild,
  getLogs,
  getSettings,
  getStats,
  listInvites,
  listUsers,
  patchSettings,
  patchUser,
  revokeInvite,
  startRebuild,
  type AdminUser,
} from "@/api/admin";
import { ApiError } from "@/api/client";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { bytesToGiB, formatUsedQuota, gibToBytes } from "@/lib/formatBytes";
import { useAuth } from "@/state/auth";

function apiMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return "Request failed.";
}

function UserEditor({
  user,
  currentUserId,
  onSaved,
  onDeleted,
}: {
  user: AdminUser;
  currentUserId: string;
  onSaved: () => void;
  onDeleted: () => void;
}) {
  const [quotaGiB, setQuotaGiB] = useState(bytesToGiB(user.storage_quota_bytes));
  const [speedKbps, setSpeedKbps] = useState(user.speed_limit_kbps != null ? String(user.speed_limit_kbps) : "");
  const [isAdmin, setIsAdmin] = useState(user.is_admin);
  const [isDisabled, setIsDisabled] = useState(user.is_disabled);
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const isSelf = user.id === currentUserId;

  const save = useMutation({
    mutationFn: async () => {
      const gib = Number(quotaGiB);
      if (!Number.isFinite(gib) || gib <= 0) throw new Error("Quota must be a positive number (GiB).");
      const quotaBytes = gibToBytes(gib);
      if (quotaBytes < user.storage_used_bytes) throw new Error("Quota cannot be below storage already used.");
      const speed = speedKbps.trim() === "" ? 0 : Number(speedKbps);
      if (speedKbps.trim() !== "" && (!Number.isFinite(speed) || speed < 0)) {
        throw new Error("Speed limit must be empty (inherit) or a non-negative Kbps value.");
      }
      await patchUser(user.id, {
        storage_quota_bytes: quotaBytes,
        speed_limit_kbps: speed,
        is_admin: isAdmin,
        is_disabled: isDisabled,
      });
    },
    onSuccess: () => {
      setError(null);
      onSaved();
    },
    onError: (e: unknown) => setError(apiMessage(e)),
  });

  const remove = useMutation({
    mutationFn: () => deleteUser(user.id),
    onSuccess: () => {
      setConfirmDelete(false);
      onDeleted();
    },
    onError: (e: unknown) => {
      setConfirmDelete(false);
      setError(apiMessage(e));
    },
  });

  return (
    <div className="explorer-user-card">
      <div className="font-medium">
        {user.username}
        {user.is_disabled ? " (disabled)" : ""}
      </div>
      <p className="mt-1 text-xs text-[#9a9a9a]">Storage: {formatUsedQuota(user.storage_used_bytes, user.storage_quota_bytes)}</p>
      <div className="mt-3 flex flex-wrap gap-4 text-xs text-[#ccc]">
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={isAdmin}
            disabled={isSelf}
            onChange={(e) => setIsAdmin(e.target.checked)}
          />
          Administrator
        </label>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={isDisabled}
            disabled={isSelf}
            onChange={(e) => setIsDisabled(e.target.checked)}
          />
          Disabled
        </label>
      </div>
      <div className="explorer-field-row mt-3">
        <label className="explorer-field">
          Quota (GiB)
          <input value={quotaGiB} onChange={(e) => setQuotaGiB(e.target.value)} inputMode="decimal" />
        </label>
        <label className="explorer-field">
          Speed limit (Kbps)
          <input
            value={speedKbps}
            onChange={(e) => setSpeedKbps(e.target.value)}
            placeholder="inherit global"
            inputMode="numeric"
          />
        </label>
        <button
          type="button"
          className="explorer-btn explorer-btn-primary"
          disabled={save.isPending}
          onClick={() => save.mutate()}
        >
          Save
        </button>
        {!isSelf ? (
          <button type="button" className="explorer-btn explorer-btn-danger" onClick={() => setConfirmDelete(true)}>
            Delete user
          </button>
        ) : null}
      </div>
      {error ? <p className="mt-2 text-xs text-red-400">{error}</p> : null}
      {confirmDelete ? (
        <ConfirmDialog
          title="Delete user"
          message={`Permanently delete “${user.username}” and all their files on disk? Revoke their unused invites first.`}
          confirmLabel="Delete user"
          danger
          onCancel={() => setConfirmDelete(false)}
          onConfirm={() => remove.mutate()}
        />
      ) : null}
    </div>
  );
}

export function AdminShellPage() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const [inviteCode, setInviteCode] = useState<string | null>(null);
  const [inviteDays, setInviteDays] = useState("");
  const [inviteQuotaGiB, setInviteQuotaGiB] = useState("");
  const [globalSpeed, setGlobalSpeed] = useState("");
  const [confirmRebuild, setConfirmRebuild] = useState(false);

  if (user && !user.is_admin) {
    return <Navigate to="/cloud" replace />;
  }

  const users = useQuery({ queryKey: ["admin", "users"], queryFn: listUsers });
  const invites = useQuery({ queryKey: ["admin", "invites"], queryFn: listInvites });
  const settings = useQuery({ queryKey: ["admin", "settings"], queryFn: getSettings });
  const stats = useQuery({ queryKey: ["admin", "stats"], queryFn: getStats });
  const logs = useQuery({ queryKey: ["admin", "logs"], queryFn: () => getLogs() });
  const rebuild = useQuery({ queryKey: ["admin", "rebuild"], queryFn: getLastRebuild });

  const saveSpeed = useMutation({
    mutationFn: () => {
      const n = globalSpeed.trim() === "" ? null : Number(globalSpeed);
      return patchSettings(n);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "settings"] }),
  });

  const runRebuild = useMutation({
    mutationFn: startRebuild,
    onSuccess: () => {
      setConfirmRebuild(false);
      qc.invalidateQueries({ queryKey: ["admin", "rebuild"] });
    },
  });

  return (
    <div className="explorer-shell">
      <div className="explorer-main">
        <div className="explorer-titlebar">
          <Link to="/cloud" className="explorer-btn">
            ← Cloud
          </Link>
          <div className="explorer-address">
            <span>Admin</span>
          </div>
        </div>

        <div className="explorer-admin-panel">
          {stats.data ? (
            <p className="text-sm text-[#9a9a9a]">
              {stats.data.user_count} users · {Math.round(stats.data.total_used_bytes / (1024 * 1024))} MiB used ·{" "}
              {stats.data.open_upload_sessions} open uploads
            </p>
          ) : null}

          <section className="explorer-admin-section">
            <h2>Global speed limit (Kbps)</h2>
            <p className="text-xs text-[#6e6e6e]">Leave empty for no global cap.</p>
            <div className="explorer-field-row">
              <label className="explorer-field">
                Limit
                <input
                  value={globalSpeed}
                  onChange={(e) => setGlobalSpeed(e.target.value)}
                  placeholder={String(settings.data?.global_speed_limit_kbps ?? "")}
                />
              </label>
              <button type="button" className="explorer-btn explorer-btn-primary" onClick={() => saveSpeed.mutate()}>
                Save
              </button>
            </div>
          </section>

          <section className="explorer-admin-section">
            <h2>Invites</h2>
            <div className="explorer-field-row">
              <label className="explorer-field">
                Expires in (days, optional)
                <input value={inviteDays} onChange={(e) => setInviteDays(e.target.value)} inputMode="numeric" />
              </label>
              <label className="explorer-field">
                Default quota (GiB, optional)
                <input value={inviteQuotaGiB} onChange={(e) => setInviteQuotaGiB(e.target.value)} inputMode="decimal" />
              </label>
              <button
                type="button"
                className="explorer-btn explorer-btn-primary"
                onClick={() => {
                  const body: { expires_at?: string; default_quota_bytes?: number } = {};
                  const days = inviteDays.trim();
                  if (days) {
                    const n = Number(days);
                    if (Number.isFinite(n) && n > 0) {
                      const exp = new Date();
                      exp.setUTCDate(exp.getUTCDate() + n);
                      body.expires_at = exp.toISOString();
                    }
                  }
                  const g = inviteQuotaGiB.trim();
                  if (g) {
                    const gib = Number(g);
                    if (Number.isFinite(gib) && gib > 0) body.default_quota_bytes = gibToBytes(gib);
                  }
                  void createInvite(body).then((r) => {
                    setInviteCode(r.code);
                    invites.refetch();
                  });
                }}
              >
                New invite
              </button>
            </div>
            {inviteCode ? (
              <p className="mb-2 rounded border border-emerald-800 bg-emerald-950/40 p-2 font-mono text-sm text-emerald-300">
                Copy now: {inviteCode}
              </p>
            ) : null}
            <ul className="explorer-list text-sm">
              {(invites.data?.items ?? []).map((i) => (
                <li key={i.id} className="explorer-list-row grid-cols-[1fr_auto]">
                  <span>
                    {i.code_hint}… {i.is_used ? "(used)" : ""}
                  </span>
                  {!i.is_used ? (
                    <button
                      type="button"
                      className="explorer-btn text-red-300"
                      onClick={() => revokeInvite(i.id).then(() => invites.refetch())}
                    >
                      Revoke
                    </button>
                  ) : null}
                </li>
              ))}
            </ul>
          </section>

          <section className="explorer-admin-section">
            <h2>Users</h2>
            {(users.data?.items ?? []).map((u) => (
              <UserEditor
                key={u.id}
                user={u}
                currentUserId={user!.id}
                onSaved={() => users.refetch()}
                onDeleted={() => {
                  users.refetch();
                  qc.invalidateQueries({ queryKey: ["admin", "stats"] });
                }}
              />
            ))}
          </section>

          <section className="explorer-admin-section">
            <h2>Backups</h2>
            <p className="text-sm text-[#9a9a9a]">
              Dumps under <code className="text-[#ccc]">/data/backups/</code> in the API container (7 kept). Restore with{" "}
              <code className="text-[#ccc]">pg_restore</code> — see <code className="text-[#ccc]">docs/OPERATOR_RUNBOOK.md</code>.
            </p>
          </section>

          <section className="explorer-admin-section">
            <div className="mb-2 flex items-center justify-between">
              <h2>Rebuild database</h2>
              <button type="button" className="explorer-btn" onClick={() => setConfirmRebuild(true)}>
                Run rebuild
              </button>
            </div>
            {rebuild.data?.finished_at ? (
              <pre className="overflow-auto rounded border border-[#333] bg-[#1a1a1a] p-3 text-xs text-[#9a9a9a]">
                {JSON.stringify(rebuild.data, null, 2)}
              </pre>
            ) : (
              <p className="text-sm text-[#6e6e6e]">No rebuild report yet.</p>
            )}
          </section>

          <section className="explorer-admin-section">
            <h2>Recent logs</h2>
            <ul className="max-h-64 overflow-y-auto explorer-list text-xs">
              {(logs.data?.items ?? []).map((l) => (
                <li key={l.id} className="explorer-list-row grid-cols-1 !block border-b border-[#2a2a2a] px-3 py-2">
                  <span className="text-[#6e6e6e]">{new Date(l.created_at).toLocaleString()}</span>{" "}
                  <span className="text-emerald-600">{l.event_type}</span> {l.message}
                </li>
              ))}
            </ul>
          </section>
        </div>
      </div>

      {confirmRebuild ? (
        <ConfirmDialog
          title="Rebuild database"
          message="Scan disk sidecars and repair metadata. This does not delete user files. Continue?"
          confirmLabel="Run rebuild"
          onCancel={() => setConfirmRebuild(false)}
          onConfirm={() => runRebuild.mutate()}
        />
      ) : null}
    </div>
  );
}
