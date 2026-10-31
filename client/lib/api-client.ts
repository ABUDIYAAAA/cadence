import axios, { AxiosError, AxiosInstance, InternalAxiosRequestConfig } from "axios";
import { ApiError } from "@/types/api";
import { getQueryClient } from "@/lib/query-client";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

// Teams moved to API v2; everything else still lives on v1.
const API_V2_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL_V2 || API_BASE_URL.replace(/\/v1\/?$/, "/v2");

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
export const apiClientV2: AxiosInstance = createClient(API_V2_BASE_URL);

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
    url.includes("/auth/refresh") ||
    url.includes("/auth/logout") ||
    url.includes("/auth/forgot-password") ||
    url.includes("/auth/setup-password") ||
    url.includes("/auth/reset-password")
  );
}

function isPublicAuthPage(pathname: string): boolean {
  return (
    pathname.startsWith("/login") ||
    pathname.startsWith("/auth/login") ||
    pathname.startsWith("/setup-password") ||
    pathname.startsWith("/auth/setup-password") ||
    pathname.startsWith("/reset-password") ||
    pathname.startsWith("/auth/reset-password") ||
    pathname.startsWith("/forgot-password") ||
    pathname.startsWith("/auth/forgot-password") ||
    pathname === "/"
  );
}

// Response interceptor: handle automatic token refresh and format errors.
// Shared by the v1 and v2 clients; a retried request keeps its own baseURL.
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
          const qc = getQueryClient();
          qc.setQueryData(["auth", "me"], null);
        } catch {}

        // If the refresh token is truly invalid/expired, redirect to login only for protected pages
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
apiClientV2.interceptors.response.use((response) => response, handleResponseError);

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
