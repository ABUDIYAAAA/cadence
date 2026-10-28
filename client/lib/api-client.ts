import axios, {
  AxiosError,
  AxiosInstance,
  InternalAxiosRequestConfig,
} from "axios";
import { ApiError } from "@/types/api";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL;

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

// Response interceptor: handle automatic token refresh and format errors
apiClient.interceptors.response.use(
  (response) => response,
  async (
    error: AxiosError<{
      error?: string;
      message?: string;
      details?: Record<string, string>;
    }>,
  ) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & {
      _retry?: boolean;
    };

    // If 401 occurs and not already retried or on auth routes (login/refresh/me/reset)
    const isAuthRoute =
      originalRequest?.url?.includes("/auth/login") ||
      originalRequest?.url?.includes("/auth/refresh") ||
      originalRequest?.url?.includes("/auth/me") ||
      originalRequest?.url?.includes("/auth/forgot-password") ||
      originalRequest?.url?.includes("/auth/setup-password") ||
      originalRequest?.url?.includes("/auth/reset-password");

    if (
      error.response?.status === 401 &&
      !originalRequest?._retry &&
      !isAuthRoute
    ) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then(() => apiClient(originalRequest))
          .catch((err) => Promise.reject(err));
      }

      originalRequest._retry = true;
      isRefreshing = true;

      try {
        await apiClient.post("/auth/refresh");
        processQueue(null);
        return apiClient(originalRequest);
      } catch (refreshErr) {
        processQueue(refreshErr);
        return Promise.reject(formatApiError(error));
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(formatApiError(error));
  },
);

export function formatApiError(
  error: AxiosError<{
    error?: string;
    message?: string;
    details?: Record<string, string>;
  }>,
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
