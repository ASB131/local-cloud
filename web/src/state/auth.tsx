import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { configureApiClient } from "@/api/client";
import * as authApi from "@/api/auth";
import type { AuthResponse, UserProfile } from "@/api/types";

const REFRESH_KEY = "localcloud.refresh_token";
const ACCESS_KEY = "localcloud.access_token";
const ACCESS_EXP_KEY = "localcloud.access_expires_at";

type AuthState = {
  user: UserProfile | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string, invite: string) => Promise<void>;
  logout: () => Promise<void>;
  accessToken: string | null;
};

const AuthContext = createContext<AuthState | null>(null);

function deviceLabel() {
  return `web-${navigator.userAgent.slice(0, 48)}`;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserProfile | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const applySession = useCallback((auth: AuthResponse) => {
    sessionStorage.setItem(REFRESH_KEY, auth.refresh_token);
    sessionStorage.setItem(ACCESS_KEY, auth.access_token);
    sessionStorage.setItem(ACCESS_EXP_KEY, auth.access_expires_at);
    setAccessToken(auth.access_token);
    setUser(auth.user);
  }, []);

  const clearSession = useCallback(() => {
    sessionStorage.removeItem(REFRESH_KEY);
    sessionStorage.removeItem(ACCESS_KEY);
    sessionStorage.removeItem(ACCESS_EXP_KEY);
    setAccessToken(null);
    setUser(null);
  }, []);

  useEffect(() => {
    configureApiClient({
      getAccessToken: () => sessionStorage.getItem(ACCESS_KEY),
      getRefreshToken: () => sessionStorage.getItem(REFRESH_KEY),
      setSession: applySession,
      clearSession,
    });
  }, [applySession, clearSession]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const refresh = sessionStorage.getItem(REFRESH_KEY);
      if (!refresh) {
        setLoading(false);
        return;
      }
      try {
        const me = await authApi.getMe();
        if (!cancelled) {
          setUser(me);
          setAccessToken(sessionStorage.getItem(ACCESS_KEY));
        }
      } catch {
        try {
          const res = await fetch("/api/v1/auth/refresh", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ refresh_token: refresh }),
          });
          if (res.ok) {
            const auth = (await res.json()) as AuthResponse;
            applySession(auth);
          } else {
            clearSession();
          }
        } catch {
          clearSession();
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [applySession, clearSession]);

  const login = useCallback(
    async (username: string, password: string) => {
      const auth = await authApi.login(username, password, deviceLabel());
      applySession(auth);
    },
    [applySession],
  );

  const register = useCallback(
    async (username: string, password: string, invite: string) => {
      const auth = await authApi.register(username, password, invite, deviceLabel());
      applySession(auth);
    },
    [applySession],
  );

  const logout = useCallback(async () => {
    const refresh = sessionStorage.getItem(REFRESH_KEY);
    if (refresh) {
      try {
        await authApi.logout(refresh);
      } catch {
        /* ignore */
      }
    }
    clearSession();
  }, [clearSession]);

  const value = useMemo(
    () => ({ user, loading, login, register, logout, accessToken }),
    [user, loading, login, register, logout, accessToken],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("AuthProvider missing");
  return ctx;
}
