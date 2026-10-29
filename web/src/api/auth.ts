import { apiRequest } from "./client";
import type { AuthResponse, UserProfile } from "./types";

export function login(username: string, password: string, deviceLabel: string) {
  return apiRequest<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    auth: false,
    body: { username, password, device_label: deviceLabel },
  });
}

export function register(
  username: string,
  password: string,
  inviteCode: string,
  deviceLabel: string,
) {
  return apiRequest<AuthResponse>("/api/v1/auth/register", {
    method: "POST",
    auth: false,
    body: { username, password, invite_code: inviteCode, device_label: deviceLabel },
  });
}

export function logout(refreshToken: string) {
  return apiRequest<void>("/api/v1/auth/logout", {
    method: "POST",
    body: { refresh_token: refreshToken },
  });
}

export function getMe() {
  return apiRequest<UserProfile>("/api/v1/auth/me");
}
