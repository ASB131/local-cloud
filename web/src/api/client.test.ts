import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiRequest, configureApiClient } from "./client";
import type { AuthResponse } from "./types";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("apiRequest", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("throws typed ApiError on failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse({ error: { code: "NOT_FOUND", message: "Missing." } }, 404),
      ),
    );
    await expect(apiRequest("/api/v1/x")).rejects.toMatchObject({
      status: 404,
      code: "NOT_FOUND",
      message: "Missing.",
    } satisfies Partial<ApiError>);
  });

  it("refreshes once on 401 and retries", async () => {
    const auth: AuthResponse = {
      access_token: "new-access",
      access_expires_at: new Date().toISOString(),
      refresh_token: "refresh",
      user: {
        id: "u",
        username: "a",
        is_admin: false,
        storage_quota_bytes: 0,
        storage_used_bytes: 0,
        speed_limit_kbps: null,
        effective_speed_limit_kbps: null,
        root_folder_id: "f",
      },
    };
    let calls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.includes("/auth/refresh")) {
          return jsonResponse(auth);
        }
        calls += 1;
        if (calls === 1) {
          return jsonResponse({ error: { code: "UNAUTHENTICATED", message: "nope" } }, 401);
        }
        return jsonResponse({ ok: true });
      }),
    );
    configureApiClient({
      getAccessToken: () => "old",
      getRefreshToken: () => "refresh",
      setSession: () => {},
      clearSession: () => {},
    });
    const out = await apiRequest<{ ok: boolean }>("/api/v1/auth/me");
    expect(out.ok).toBe(true);
    expect(calls).toBe(2);
  });
});
