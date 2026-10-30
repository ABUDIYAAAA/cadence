export interface ApiError {
  error: string;
  details?: Record<string, string>;
  status?: number;
  /** Raw JSON error body, for endpoints that return structured rejections (e.g. scans). */
  body?: Record<string, unknown>;
}

export interface PaginationParams {
  page?: number;
  limit?: number;
  search?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total_count: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface SimpleMessageResponse {
  message: string;
}
