import axios, { type AxiosError, type AxiosInstance, type InternalAxiosRequestConfig } from "axios";
import type { ApiError } from "../types/api.ts";
import { getQueryClient } from "./query-client.ts";
import {
  buildScopedQueryKey,
  clearTenantScope,
  getSavedCompetitionForEvent,
  getTenantScope,
  isScopeReady,
  resolveScopedPath,
  setTenantScope,
  subscribeTenantScope,
  type ScopeLevel,
  type TenantScope,
} from "./tenant-scope.ts";

export {
  buildScopedQueryKey,
  clearTenantScope,
  getSavedCompetitionForEvent,
  getTenantScope,
  isScopeReady,
  resolveScopedPath,
  setTenantScope,
  subscribeTenantScope,
};
export type { ScopeLevel, TenantScope };

const rawBaseUrl =
  process.env.NEXT_PUBLIC_API_URL_V2 ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080/api/v2";

// Unified API v2 base URL (auto-upgrades legacy /v1 env values to /v2).
export const API_BASE_URL = rawBaseUrl.replace(/\/v1\/?$/, "/v2");

function createClient(baseURL: string): AxiosInstance {
  return axios.create({
    baseURL,
    withCredentials: true,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
  });
}

export const apiClient: AxiosInstance = createClient(API_BASE_URL);
export const apiClientV2: AxiosInstance = apiClient;

// Request interceptor: rewrite relative feature paths to active event/competition scope.
apiClient.interceptors.request.use((config) => {
  const skipScope = (config as InternalAxiosRequestConfig & { skipTenantScope?: boolean })
    .skipTenantScope;
  if (config.url && !skipScope) {
    config.url = resolveScopedPath(config.url, getTenantScope());
  }
  return config;
});

let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value?: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

const processQueue = (error: unknown) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve();
    }
  });
  failedQueue = [];
};

/**
 * Endpoints that should NOT trigger a token refresh when they return 401.
 * /auth/me MUST NOT be in this list because when access token expires,
 * /auth/me must seamlessly refresh via /auth/refresh and retry!
 */
function isNonRefreshableAuthRoute(url?: string): boolean {
  if (!url) return false;
  return (
    url.includes("/auth/login") ||
    url.includes("/auth/google/login") ||
    url.includes("/auth/google/callback") ||
    url.includes("/auth/refresh") ||
    url.includes("/auth/logout")
  );
}

function isPublicAuthPage(pathname: string): boolean {
  return (
    pathname.startsWith("/login") ||
    pathname.startsWith("/auth/login") ||
    pathname.startsWith("/auth/callback") ||
    pathname === "/"
  );
}

const handleResponseError = async (
  error: AxiosError<{
    error?: string;
    message?: string;
    details?: Record<string, string>;
  }>
) => {
  const originalRequest = error.config as InternalAxiosRequestConfig & {
    _retry?: boolean;
  };

  if (
    error.response?.status === 401 &&
    originalRequest &&
    !originalRequest._retry &&
    !isNonRefreshableAuthRoute(originalRequest.url)
  ) {
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failedQueue.push({ resolve, reject });
      })
        .then(() => {
          originalRequest._retry = true;
          return apiClient(originalRequest);
        })
        .catch((err) => Promise.reject(err));
    }

    originalRequest._retry = true;
    isRefreshing = true;

    try {
      // Direct call without interceptors to avoid circular refresh loops
      await axios.post(
        `${API_BASE_URL}/auth/refresh`,
        {},
        { withCredentials: true }
      );

      processQueue(null);
      return apiClient(originalRequest);
    } catch (refreshErr) {
      processQueue(refreshErr);

      // Mark user as unauthenticated in cache.
      // NOTE: NEVER call qc.clear() here because active mounted query observers
      // (like useCurrentUser in AuthProvider) will instantly refetch /auth/me,
      // creating a rapid 35ms infinite refresh loop!
      try {
        clearTenantScope();
        const qc = getQueryClient();
        qc.setQueryData(["auth", "me"], null);
      } catch {}

      if (
        typeof window !== "undefined" &&
        !isPublicAuthPage(window.location.pathname)
      ) {
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination
        window.location.href = "/login";
      }

      return Promise.reject(formatApiError(error));
    } finally {
      isRefreshing = false;
    }
  }

  return Promise.reject(formatApiError(error));
};

apiClient.interceptors.response.use((response) => response, handleResponseError);

export function formatApiError(
  error: AxiosError<{
    error?: string;
    message?: string;
    details?: Record<string, string>;
  }>
): ApiError {
  if (error.response?.data) {
    return {
      error:
        error.response.data.error ||
        error.response.data.message ||
        "An unexpected error occurred",
      details: error.response.data.details,
      status: error.response.status,
      body:
        typeof error.response.data === "object"
          ? (error.response.data as Record<string, unknown>)
          : undefined,
    };
  }

  if (error.request) {
    return {
      error: "Unable to reach server. Please check your network connection.",
      status: 0,
    };
  }

  return {
    error: error.message || "An unexpected error occurred",
  };
}
