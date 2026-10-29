import type { ApiErrorBody, AuthResponse } from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: Record<string, unknown>;

  constructor(status: number, code: string, message: string, details?: Record<string, unknown>) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export type TokenStore = {
  getAccessToken: () => string | null;
  getRefreshToken: () => string | null;
  setSession: (auth: AuthResponse) => void;
  clearSession: () => void;
};

type RequestOptions = Omit<RequestInit, "body"> & {
  body?: unknown;
  auth?: boolean;
  rawBody?: BodyInit | null;
};

let tokenStore: TokenStore | null = null;
let refreshInFlight: Promise<AuthResponse | null> | null = null;

export function configureApiClient(store: TokenStore) {
  tokenStore = store;
}

async function parseError(res: Response): Promise<ApiError> {
  let code = "INTERNAL";
  let message = res.statusText || "Request failed";
  let details: Record<string, unknown> | undefined;
  try {
    const data = (await res.json()) as ApiErrorBody;
    code = data.error?.code ?? code;
    message = data.error?.message ?? message;
    details = data.error?.details;
  } catch {
    /* non-json */
  }
  return new ApiError(res.status, code, message, details);
}

async function refreshAccessToken(): Promise<AuthResponse | null> {
  if (!tokenStore) return null;
  const refresh = tokenStore.getRefreshToken();
  if (!refresh) return null;
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
        const res = await fetch("/api/v1/auth/refresh", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refresh }),
        });
        if (!res.ok) {
          tokenStore?.clearSession();
          return null;
        }
        const auth = (await res.json()) as AuthResponse;
        tokenStore?.setSession(auth);
        return auth;
      } finally {
        refreshInFlight = null;
      }
    })();
  }
  return refreshInFlight;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const useAuth = options.auth !== false;
  if (useAuth && tokenStore) {
    const token = tokenStore.getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  if (options.body !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const init: RequestInit = {
    ...options,
    headers,
    body:
      options.rawBody !== undefined
        ? options.rawBody
        : options.body !== undefined
          ? JSON.stringify(options.body)
          : undefined,
  };

  let res = await fetch(path, init);
  if (res.status === 401 && useAuth && tokenStore?.getRefreshToken()) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      headers.set("Authorization", `Bearer ${refreshed.access_token}`);
      res = await fetch(path, { ...init, headers });
    }
  }

  if (res.status === 204) {
    return undefined as T;
  }
  if (!res.ok) {
    throw await parseError(res);
  }
  const ct = res.headers.get("Content-Type") ?? "";
  if (ct.includes("application/json")) {
    return (await res.json()) as T;
  }
  return (await res.text()) as T;
}

export async function apiDownloadPost(path: string, body: unknown): Promise<Blob> {
  const headers = new Headers({ "Content-Type": "application/json" });
  if (tokenStore) {
    const token = tokenStore.getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  let res = await fetch(path, { method: "POST", headers, body: JSON.stringify(body) });
  if (res.status === 401 && tokenStore?.getRefreshToken()) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      headers.set("Authorization", `Bearer ${refreshed.access_token}`);
      res = await fetch(path, { method: "POST", headers, body: JSON.stringify(body) });
    }
  }
  if (!res.ok) throw await parseError(res);
  return res.blob();
}

export async function apiDownloadBlob(path: string): Promise<Blob> {
  const headers = new Headers();
  if (tokenStore) {
    const token = tokenStore.getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  let res = await fetch(path, { headers });
  if (res.status === 401 && tokenStore?.getRefreshToken()) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      headers.set("Authorization", `Bearer ${refreshed.access_token}`);
      res = await fetch(path, { headers });
    }
  }
  if (!res.ok) throw await parseError(res);
  return res.blob();
}

export async function apiUploadChunk(
  sessionId: string,
  offset: number,
  body: Blob,
  signal?: AbortSignal,
): Promise<{ received_bytes: number }> {
  const headers = new Headers();
  headers.set("Offset", String(offset));
  if (tokenStore) {
    const token = tokenStore.getAccessToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  let res = await fetch(`/api/v1/uploads/${sessionId}/content`, {
    method: "PUT",
    headers,
    body,
    signal,
  });
  if (res.status === 401 && tokenStore?.getRefreshToken()) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      headers.set("Authorization", `Bearer ${refreshed.access_token}`);
      res = await fetch(`/api/v1/uploads/${sessionId}/content`, {
        method: "PUT",
        headers,
        body,
        signal,
      });
    }
  }
  if (!res.ok) throw await parseError(res);
  return (await res.json()) as { received_bytes: number };
}
