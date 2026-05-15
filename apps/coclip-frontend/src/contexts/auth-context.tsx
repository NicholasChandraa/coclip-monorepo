"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { AUTH_BASE, ENGINE_BASE } from "@/lib/api";

export interface User {
  id: string;
  username: string;
  email: string;
  full_name: string;
  is_active: boolean;
  roles: string[];
  permissions: string[];
}

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (
    username: string,
    email: string,
    password: string,
    fullName: string
  ) => Promise<void>;
  logout: () => Promise<void>;
  engineFetch: (path: string, options?: RequestInit) => Promise<Response>;
  authFetch: (path: string, options?: RequestInit) => Promise<Response>;
  updateUser: () => Promise<void>;
  getToken: () => string | null;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  // Access token disimpan di memory saja (tidak di localStorage)
  // Cookie access_token di-set otomatis oleh auth-service (HttpOnly)
  const tokenRef = useRef<string | null>(null);

  const clearAuth = useCallback(() => {
    tokenRef.current = null;
    setUser(null);
  }, []);

  const fetchMe = useCallback(async (): Promise<User | null> => {
    if (!tokenRef.current) return null;
    try {
      const res = await fetch(`${AUTH_BASE}/auth/me`, {
        credentials: "include",
        headers: { Authorization: `Bearer ${tokenRef.current}` },
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }, []);

  const refreshAccessToken = useCallback(async (): Promise<boolean> => {
    try {
      const res = await fetch(`${AUTH_BASE}/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) return false;
      const data = await res.json();
      // Simpan di memory untuk auth-service calls; cookie di-set otomatis oleh server
      tokenRef.current = data.access_token;
      return true;
    } catch {
      return false;
    }
  }, []);

  // Initialize: coba silent refresh via HttpOnly cookie pada mount
  useEffect(() => {
    const init = async () => {
      try {
        const ok = await refreshAccessToken();
        if (ok) {
          const userData = await fetchMe();
          if (userData) setUser(userData);
          else clearAuth();
        } else {
          clearAuth();
        }
      } catch {
        clearAuth();
      } finally {
        setIsLoading(false);
      }
    };
    init();
  }, [fetchMe, refreshAccessToken, clearAuth]);

  const login = useCallback(
    async (username: string, password: string) => {
      const res = await fetch(`${AUTH_BASE}/auth/token`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Login failed");
      }
      const data = await res.json();
      // Simpan di memory untuk auth-service calls; cookie di-set otomatis oleh server
      tokenRef.current = data.access_token;
      const userData = await fetchMe();
      if (userData) setUser(userData);
    },
    [fetchMe]
  );

  const register = useCallback(
    async (
      username: string,
      email: string,
      password: string,
      fullName: string
    ) => {
      const res = await fetch(`${AUTH_BASE}/auth/register`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username,
          email,
          password,
          full_name: fullName,
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Registration failed");
      }
    },
    []
  );

  const logout = useCallback(async () => {
    try {
      await fetch(`${AUTH_BASE}/auth/logout`, {
        method: "POST",
        credentials: "include",
        headers: tokenRef.current
          ? { Authorization: `Bearer ${tokenRef.current}` }
          : {},
      });
    } finally {
      clearAuth();
    }
  }, [clearAuth]);

  const updateUser = useCallback(async () => {
    const userData = await fetchMe();
    if (userData) setUser(userData);
  }, [fetchMe]);

  const getToken = useCallback(() => tokenRef.current, []);

  // Fetch wrapper untuk auth-service — pakai Authorization header dari tokenRef
  // Auto-refresh jika 401
  const authFetch = useCallback(
    async (path: string, options?: RequestInit): Promise<Response> => {
      const doFetch = () =>
        fetch(`${AUTH_BASE}${path}`, {
          ...options,
          credentials: "include",
          headers: {
            ...options?.headers,
            ...(tokenRef.current
              ? { Authorization: `Bearer ${tokenRef.current}` }
              : {}),
          },
        });

      let res = await doFetch();

      if (res.status === 401) {
        const ok = await refreshAccessToken();
        if (!ok) {
          clearAuth();
          throw new Error("Session expired. Please login again.");
        }
        res = await doFetch();
      }

      return res;
    },
    [refreshAccessToken, clearAuth]
  );

  // Fetch wrapper untuk engine — kirim Authorization header (untuk cross-origin/tunnel)
  // + credentials include sebagai fallback untuk same-origin/local dev
  // Auto-refresh jika engine return 401
  const engineFetch = useCallback(
    async (path: string, options?: RequestInit): Promise<Response> => {
      const doFetch = () =>
        fetch(`${ENGINE_BASE}${path}`, {
          ...options,
          credentials: "include",
          headers: {
            ...options?.headers,
            ...(tokenRef.current
              ? { Authorization: `Bearer ${tokenRef.current}` }
              : {}),
          },
        });

      let res = await doFetch();

      if (res.status === 401) {
        // Access token cookie expired — coba refresh
        const ok = await refreshAccessToken();
        if (!ok) {
          clearAuth();
          throw new Error("Session expired. Please login again.");
        }
        // Cookie access_token sudah diperbarui oleh server, retry request
        res = await doFetch();
      }

      return res;
    },
    [refreshAccessToken, clearAuth]
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        login,
        register,
        logout,
        engineFetch,
        authFetch,
        updateUser,
        getToken,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
