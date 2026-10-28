import axios, { AxiosError, AxiosInstance, InternalAxiosRequestConfig } from "axios";
import { ApiError } from "@/types/api";
import { getQueryClient } from "@/lib/query-client";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
  },
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
    url.includes("/auth/refresh") ||
    url.includes("/auth/logout") ||
    url.includes("/auth/forgot-password") ||
    url.includes("/auth/setup-password") ||
    url.includes("/auth/reset-password")
  );
}

// Response interceptor: handle automatic token refresh and format errors
apiClient.interceptors.response.use(
  (response) => response,
  async (
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

        // Clear cached auth/query state to prevent redirect ping-pong loops
        try {
          const qc = getQueryClient();
          qc.setQueryData(["auth", "me"], null);
          qc.clear();
        } catch {}

        // If the refresh token is truly invalid/expired, redirect to login once
        if (
          typeof window !== "undefined" &&
          !window.location.pathname.startsWith("/login") &&
          !window.location.pathname.startsWith("/auth/login")
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
  }
);

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
