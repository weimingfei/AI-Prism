import axios, {
  AxiosError,
  AxiosHeaders,
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
} from "axios";
import { resolveAppEnv } from "@/config/env";
import {
  AppError,
  ErrorCode,
  ErrorMessages,
  localizeErrorMessage,
  type ErrorCode as ErrorCodeValue,
} from "@/lib/errors";
import { getAuthToken } from "@/lib/authToken";
import { resolveActiveAiProviderHeaders } from "@/lib/aiProviderSettings";

export interface BaseResponse<T = unknown> {
  code: string;
  message: string | null;
  data: T;
  requestId: string | null;
  success: boolean;
}

export type ApiResponse<T = unknown> = BaseResponse<T>;

export type RequestDedupeStrategy =
  | "off"
  | "join"
  | "cancel-previous"
  | "reject";

export type RequestPolicy = {
  key?: string;
  dedupe?: RequestDedupeStrategy;
  debounceMs?: number;
};

export type AppRequestConfig<D = unknown> = AxiosRequestConfig<D> & {
  requestPolicy?: RequestPolicy;
};

export interface HttpClient {
  get<T>(url: string, config?: AppRequestConfig): Promise<T>;
  delete<T>(url: string, config?: AppRequestConfig): Promise<T>;
  post<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ): Promise<T>;
  put<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ): Promise<T>;
  patch<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ): Promise<T>;
}

export const getApiBaseUrl = () => resolveAppEnv().apiBaseUrl;

const AUTH_FREE_API_PATHS = new Set([
  "/lingzhi/v1/users/login",
  "/lingzhi/v1/users/register",
  "/lingzhi/v1/users/check-login",
]);

const trimQueryAndHash = (path: string) => {
  const queryIndex = path.indexOf("?");
  const hashIndex = path.indexOf("#");
  const stopIndexes = [queryIndex, hashIndex].filter((index) => index >= 0);
  if (stopIndexes.length === 0) {
    return path;
  }
  return path.slice(0, Math.min(...stopIndexes));
};

const normalizeRequestPath = (url?: string) => {
  if (!url) {
    return "";
  }

  if (url.startsWith("http://") || url.startsWith("https://")) {
    try {
      const parsed = new URL(url);
      return trimQueryAndHash(parsed.pathname || "");
    } catch {
      return "";
    }
  }

  const baseUrl = getApiBaseUrl();
  const normalizedRelative = url.startsWith("/") ? url : `/${url}`;
  if (normalizedRelative.startsWith(baseUrl)) {
    return trimQueryAndHash(normalizedRelative.slice(baseUrl.length));
  }

  return trimQueryAndHash(normalizedRelative);
};

export const requiresAuthTokenForRequest = (url?: string) => {
  const path = normalizeRequestPath(url);
  if (!path.startsWith("/lingzhi/v1/")) {
    return false;
  }
  return !AUTH_FREE_API_PATHS.has(path);
};

export const assertRequestAuthorized = (url: string | undefined) => {
  const token = getAuthToken();
  if (requiresAuthTokenForRequest(url) && !token) {
    throw new AppError(
      ErrorCode.UNAUTHORIZED,
      ErrorMessages[ErrorCode.UNAUTHORIZED],
    );
  }
  return token;
};

export const buildApiUrl = (
  path: string,
  query?: Record<string, string | number | boolean | null | undefined>,
) => {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const baseUrl = `${getApiBaseUrl()}${normalizedPath}`;

  if (!query) {
    return baseUrl;
  }

  const searchParams = new URLSearchParams();
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") {
      return;
    }
    searchParams.set(key, String(value));
  });

  const queryString = searchParams.toString();
  return queryString ? `${baseUrl}?${queryString}` : baseUrl;
};

export const mapBusinessCode = (code: string | undefined): ErrorCodeValue => {
  switch (code) {
    case "401":
      return ErrorCode.UNAUTHORIZED;
    case "403":
      return ErrorCode.FORBIDDEN;
    default:
      return ErrorCode.OPERATION_FAILED;
  }
};

export const isBaseResponse = <T>(
  payload: unknown,
): payload is BaseResponse<T> => {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  return "data" in payload && ("code" in payload || "success" in payload);
};

export const unwrapResponseData = <T>(payload: BaseResponse<T> | T): T => {
  if (isBaseResponse<T>(payload)) {
    if (payload.code === "0" || payload.success) {
      return payload.data;
    }

    throw new AppError(
      mapBusinessCode(payload.code),
      localizeErrorMessage(payload.message, "请求失败，请稍后重试。"),
      payload,
    );
  }

  return payload;
};

export const unwrapResponse = <T>(
  response: AxiosResponse<BaseResponse<T> | T>,
): T => {
  return unwrapResponseData(response.data);
};

export const mapAxiosErrorToAppError = (error: AxiosError): AppError => {
  if (axios.isCancel(error)) {
    return new AppError(
      ErrorCode.ABORTED,
      ErrorMessages[ErrorCode.ABORTED],
      error,
    );
  }

  if (error.response) {
    const responseMessage =
      isBaseResponse(error.response.data) &&
      typeof error.response.data.message === "string" &&
      error.response.data.message.trim()
        ? error.response.data.message.trim()
        : undefined;
    switch (error.response.status) {
      case 400:
        return new AppError(
          ErrorCode.INVALID_PARAMS,
          localizeErrorMessage(
            responseMessage,
            "请求参数不正确，请检查后重试。",
          ),
          error,
        );
      case 401:
        return new AppError(
          ErrorCode.UNAUTHORIZED,
          localizeErrorMessage(
            responseMessage,
            ErrorMessages[ErrorCode.UNAUTHORIZED],
          ),
          error,
        );
      case 403:
        return new AppError(
          ErrorCode.FORBIDDEN,
          localizeErrorMessage(
            responseMessage,
            ErrorMessages[ErrorCode.FORBIDDEN],
          ),
          error,
        );
      case 404:
        return new AppError(
          ErrorCode.RESOURCE_NOT_FOUND,
          localizeErrorMessage(
            responseMessage,
            ErrorMessages[ErrorCode.RESOURCE_NOT_FOUND],
          ),
          error,
        );
      case 408:
        return new AppError(
          ErrorCode.REQUEST_TIMEOUT,
          localizeErrorMessage(
            responseMessage,
            ErrorMessages[ErrorCode.REQUEST_TIMEOUT],
          ),
          error,
        );
      case 413:
        return new AppError(
          ErrorCode.FILE_UPLOAD_ERROR,
          localizeErrorMessage(
            responseMessage,
            "上传内容过大，请压缩文件或联系管理员调整上传限制。",
          ),
          error,
        );
      case 415:
        return new AppError(
          ErrorCode.INVALID_PARAMS,
          localizeErrorMessage(
            responseMessage,
            "上传格式不支持，请更换文件后重试。",
          ),
          error,
        );
      case 429:
        return new AppError(
          ErrorCode.OPERATION_FAILED,
          localizeErrorMessage(responseMessage, "请求过于频繁，请稍后再试。"),
          error,
        );
      case 500:
        return new AppError(
          ErrorCode.UNKNOWN_ERROR,
          localizeErrorMessage(responseMessage, "服务器内部错误，请稍后重试。"),
          error,
        );
      default:
        return new AppError(
          ErrorCode.UNKNOWN_ERROR,
          localizeErrorMessage(
            responseMessage,
            `请求失败，服务器返回状态码 ${error.response.status}。`,
          ),
          error,
        );
    }
  }

  if (error.code === "ECONNABORTED") {
    return new AppError(
      ErrorCode.REQUEST_TIMEOUT,
      ErrorMessages[ErrorCode.REQUEST_TIMEOUT],
      error,
    );
  }

  if (typeof window !== "undefined" && !window.navigator.onLine) {
    return new AppError(
      ErrorCode.NETWORK_ERROR,
      "网络已断开，请检查网络连接。",
      error,
    );
  }

  return new AppError(
    ErrorCode.NETWORK_ERROR,
    localizeErrorMessage(error.message, ErrorMessages[ErrorCode.NETWORK_ERROR]),
    error,
  );
};

const stableSerialize = (
  value: unknown,
  seen = new WeakSet<object>(),
): string => {
  if (value === null || value === undefined) {
    return String(value);
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return JSON.stringify(value);
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (value instanceof URLSearchParams) {
    return value.toString();
  }
  if (typeof FormData !== "undefined" && value instanceof FormData) {
    return "[form-data]";
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableSerialize(item, seen)).join(",")}]`;
  }
  if (typeof value === "object") {
    if (seen.has(value as object)) {
      return "[circular]";
    }
    seen.add(value as object);
    const record = value as Record<string, unknown>;
    const serialized = Object.keys(record)
      .sort()
      .map((key) => `${key}:${stableSerialize(record[key], seen)}`)
      .join(",");
    seen.delete(value as object);
    return `{${serialized}}`;
  }
  return String(value);
};

export const buildRequestPolicyKey = (args: {
  method: string;
  url: string;
  params?: unknown;
  data?: unknown;
  explicitKey?: string;
}) => {
  const normalizedExplicitKey = args.explicitKey?.trim();
  if (normalizedExplicitKey) {
    return normalizedExplicitKey;
  }
  const normalizedMethod = args.method.toUpperCase();
  const normalizedPath = normalizeRequestPath(args.url) || args.url;
  const serializedParams = stableSerialize(args.params);
  const serializedData = stableSerialize(args.data);
  return `${normalizedMethod}::${normalizedPath}::params=${serializedParams}::data=${serializedData}`;
};

export const resolveRequestPolicy = (
  method: string,
  requestPolicy?: RequestPolicy,
) => {
  const normalizedMethod = method.trim().toUpperCase();
  const dedupe: RequestDedupeStrategy =
    requestPolicy?.dedupe ?? (normalizedMethod === "GET" ? "join" : "off");
  const debounceMsRaw = requestPolicy?.debounceMs;
  const debounceMs =
    typeof debounceMsRaw === "number" && Number.isFinite(debounceMsRaw)
      ? Math.max(0, Math.floor(debounceMsRaw))
      : 0;

  return {
    dedupe,
    debounceMs,
    key: requestPolicy?.key?.trim() || undefined,
  };
};

type InflightRequestEntry<T = unknown> = {
  controller: AbortController;
  promise: Promise<T>;
};

type DebounceResolver<T> = {
  resolve: (value: T) => void;
  reject: (error: unknown) => void;
};

type DebounceRequestEntry<T = unknown> = {
  timerId: ReturnType<typeof setTimeout> | null;
  run: () => Promise<T>;
  resolvers: DebounceResolver<T>[];
};

const inflightRequestMap = new Map<string, InflightRequestEntry<unknown>>();
const debounceRequestMap = new Map<string, DebounceRequestEntry<unknown>>();

const bindAbortSignal = (
  controller: AbortController,
  signal?: AxiosRequestConfig["signal"],
) => {
  if (!signal) {
    return;
  }
  if (signal.aborted) {
    controller.abort();
    return;
  }
  if (typeof (signal as AbortSignal).addEventListener === "function") {
    (signal as AbortSignal).addEventListener(
      "abort",
      () => controller.abort(),
      {
        once: true,
      },
    );
  }
};

const stripRequestPolicy = <D>(
  config?: AppRequestConfig<D>,
): AxiosRequestConfig<D> => {
  if (!config) {
    return {};
  }
  const axiosConfig = { ...config } as AxiosRequestConfig<D> & {
    requestPolicy?: RequestPolicy;
  };
  delete axiosConfig.requestPolicy;
  return axiosConfig;
};

const withDebounce = <T>(
  requestKey: string,
  debounceMs: number,
  run: () => Promise<T>,
): Promise<T> => {
  if (debounceMs <= 0) {
    return run();
  }

  const existing = debounceRequestMap.get(requestKey) as
    | DebounceRequestEntry<T>
    | undefined;

  if (existing) {
    if (existing.timerId !== null) {
      clearTimeout(existing.timerId);
    }
    existing.run = run;
    return new Promise<T>((resolve, reject) => {
      existing.resolvers.push({ resolve, reject });
      existing.timerId = setTimeout(async () => {
        const active = debounceRequestMap.get(requestKey) as
          | DebounceRequestEntry<T>
          | undefined;
        if (!active) {
          return;
        }
        debounceRequestMap.delete(requestKey);
        try {
          const result = await active.run();
          active.resolvers.forEach((entry) => entry.resolve(result));
        } catch (error) {
          active.resolvers.forEach((entry) => entry.reject(error));
        }
      }, debounceMs);
    });
  }

  const created: DebounceRequestEntry<T> = {
    timerId: null,
    run,
    resolvers: [],
  };
  debounceRequestMap.set(requestKey, created as DebounceRequestEntry<unknown>);

  return new Promise<T>((resolve, reject) => {
    created.resolvers.push({ resolve, reject });
    created.timerId = setTimeout(async () => {
      const active = debounceRequestMap.get(requestKey) as
        | DebounceRequestEntry<T>
        | undefined;
      if (!active) {
        return;
      }
      debounceRequestMap.delete(requestKey);
      try {
        const result = await active.run();
        active.resolvers.forEach((entry) => entry.resolve(result));
      } catch (error) {
        active.resolvers.forEach((entry) => entry.reject(error));
      }
    }, debounceMs);
  });
};

const withInflightDedup = <T, D>(args: {
  requestKey: string;
  dedupe: RequestDedupeStrategy;
  config?: AxiosRequestConfig<D>;
  run: (config: AxiosRequestConfig<D>) => Promise<T>;
}) => {
  const existing = inflightRequestMap.get(args.requestKey) as
    | InflightRequestEntry<T>
    | undefined;

  if (existing) {
    if (args.dedupe === "join") {
      return existing.promise;
    }
    if (args.dedupe === "reject") {
      throw new AppError(
        ErrorCode.OPERATION_FAILED,
        "已有相同请求正在处理中，请稍后重试。",
      );
    }
    if (args.dedupe === "cancel-previous") {
      existing.controller.abort();
    }
  }

  const controller = new AbortController();
  bindAbortSignal(controller, args.config?.signal);
  const mergedConfig: AxiosRequestConfig<D> = {
    ...(args.config ?? {}),
    signal: controller.signal,
  };

  const promise = args.run(mergedConfig).finally(() => {
    const active = inflightRequestMap.get(args.requestKey);
    if (active?.promise === promise) {
      inflightRequestMap.delete(args.requestKey);
    }
  });

  if (args.dedupe !== "off") {
    inflightRequestMap.set(args.requestKey, {
      controller,
      promise,
    } as InflightRequestEntry<unknown>);
  }

  return promise;
};

export const createAxiosClient = (): AxiosInstance => {
  return axios.create({
    baseURL: getApiBaseUrl(),
    timeout: 10_000,
    headers: {
      "Content-Type": "application/json;charset=utf-8",
    },
  });
};

const axiosInstance = createAxiosClient();

axiosInstance.interceptors.request.use(
  (config) => {
    const token = assertRequestAuthorized(config.url);
    const headers = new AxiosHeaders(config.headers);

    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const providerHeaders = resolveActiveAiProviderHeaders();
    if (providerHeaders) {
      Object.entries(providerHeaders).forEach(([key, value]) => {
        if (value.trim()) {
          headers.set(key, value);
        }
      });
    }

    config.headers = headers;

    return config;
  },
  (error) =>
    Promise.reject(
      new AppError(
        ErrorCode.UNKNOWN_ERROR,
        "请求发送失败，请稍后重试。",
        error,
      ),
    ),
);

axiosInstance.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => Promise.reject(mapAxiosErrorToAppError(error)),
);

const executeWithPolicy = <T, D>(args: {
  method: "GET" | "DELETE" | "POST" | "PUT" | "PATCH";
  url: string;
  data?: D;
  config?: AppRequestConfig<D>;
  run: (config: AxiosRequestConfig<D>) => Promise<T>;
}) => {
  const policy = resolveRequestPolicy(args.method, args.config?.requestPolicy);
  const requestKey = buildRequestPolicyKey({
    method: args.method,
    url: args.url,
    params: args.config?.params,
    data: args.data,
    explicitKey: policy.key,
  });
  const axiosConfig = stripRequestPolicy(args.config);

  const runWithDedupe = () =>
    withInflightDedup({
      requestKey,
      dedupe: policy.dedupe,
      config: axiosConfig,
      run: args.run,
    });

  return withDebounce(requestKey, policy.debounceMs, runWithDedupe);
};

const service: HttpClient = {
  async get<T>(url: string, config?: AppRequestConfig) {
    return executeWithPolicy<T, unknown>({
      method: "GET",
      url,
      config,
      run: async (axiosConfig) => {
        const response = await axiosInstance.get<BaseResponse<T> | T>(
          url,
          axiosConfig,
        );
        return unwrapResponse(response);
      },
    });
  },
  async delete<T>(url: string, config?: AppRequestConfig) {
    return executeWithPolicy<T, unknown>({
      method: "DELETE",
      url,
      config,
      run: async (axiosConfig) => {
        const response = await axiosInstance.delete<BaseResponse<T> | T>(
          url,
          axiosConfig,
        );
        return unwrapResponse(response);
      },
    });
  },
  async post<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ) {
    return executeWithPolicy<T, D>({
      method: "POST",
      url,
      data,
      config,
      run: async (axiosConfig) => {
        const response = await axiosInstance.post<BaseResponse<T> | T>(
          url,
          data,
          axiosConfig,
        );
        return unwrapResponse(response);
      },
    });
  },
  async put<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ) {
    return executeWithPolicy<T, D>({
      method: "PUT",
      url,
      data,
      config,
      run: async (axiosConfig) => {
        const response = await axiosInstance.put<BaseResponse<T> | T>(
          url,
          data,
          axiosConfig,
        );
        return unwrapResponse(response);
      },
    });
  },
  async patch<T, D = unknown>(
    url: string,
    data?: D,
    config?: AppRequestConfig<D>,
  ) {
    return executeWithPolicy<T, D>({
      method: "PATCH",
      url,
      data,
      config,
      run: async (axiosConfig) => {
        const response = await axiosInstance.patch<BaseResponse<T> | T>(
          url,
          data,
          axiosConfig,
        );
        return unwrapResponse(response);
      },
    });
  },
};

export default service;
