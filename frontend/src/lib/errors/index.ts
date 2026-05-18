export const ErrorCode = {
  // 通用错误（1000-1999）
  UNKNOWN_ERROR: 1000,
  NETWORK_ERROR: 1001,
  REQUEST_TIMEOUT: 1002,
  ABORTED: 1003,

  // 登录鉴权错误（2000-2999）
  UNAUTHORIZED: 2001,
  FORBIDDEN: 2002,
  TOKEN_EXPIRED: 2003,

  // 业务错误（3000-3999）
  INVALID_PARAMS: 3001,
  RESOURCE_NOT_FOUND: 3002,
  OPERATION_FAILED: 3003,

  // AI 调用错误（4000-4999）
  AI_SERVICE_UNAVAILABLE: 4001,
  AI_QUOTA_EXCEEDED: 4002,
  AI_RESPONSE_ERROR: 4003,
  AI_STREAM_PARSING_ERROR: 4004,

  // 客户端错误（5000-5999）
  CLIENT_VALIDATION_ERROR: 5001,
  FILE_UPLOAD_ERROR: 5002,
  CAMERA_PERMISSION_DENIED: 5003,
  MICROPHONE_PERMISSION_DENIED: 5004,
} as const;

export type ErrorCode = (typeof ErrorCode)[keyof typeof ErrorCode];

export const ErrorMessages: Record<ErrorCode, string> = {
  [ErrorCode.UNKNOWN_ERROR]: "发生未知错误，请稍后重试。",
  [ErrorCode.NETWORK_ERROR]: "网络连接失败，请检查网络后重试。",
  [ErrorCode.REQUEST_TIMEOUT]: "请求超时，请稍后重试。",
  [ErrorCode.ABORTED]: "请求已取消。",

  [ErrorCode.UNAUTHORIZED]: "登录状态已失效，请重新登录。",
  [ErrorCode.FORBIDDEN]: "没有权限执行此操作。",
  [ErrorCode.TOKEN_EXPIRED]: "登录凭证已过期，请重新登录。",

  [ErrorCode.INVALID_PARAMS]: "请求参数不正确，请检查后重试。",
  [ErrorCode.RESOURCE_NOT_FOUND]: "请求的资源不存在。",
  [ErrorCode.OPERATION_FAILED]: "操作失败，请稍后重试。",

  [ErrorCode.AI_SERVICE_UNAVAILABLE]: "AI 服务暂时不可用，请稍后重试。",
  [ErrorCode.AI_QUOTA_EXCEEDED]: "AI 调用额度不足。",
  [ErrorCode.AI_RESPONSE_ERROR]: "AI 响应失败，请稍后重试。",
  [ErrorCode.AI_STREAM_PARSING_ERROR]: "解析 AI 流式响应失败。",

  [ErrorCode.CLIENT_VALIDATION_ERROR]: "输入内容校验失败，请检查后重试。",
  [ErrorCode.FILE_UPLOAD_ERROR]: "文件上传失败，请稍后重试。",
  [ErrorCode.CAMERA_PERMISSION_DENIED]:
    "摄像头权限被拒绝，请检查浏览器权限设置。",
  [ErrorCode.MICROPHONE_PERMISSION_DENIED]:
    "麦克风权限被拒绝，请检查浏览器权限设置。",
};

const messageTranslations: Array<[RegExp, string]> = [
  [/^Request was cancelled\.?$/i, ErrorMessages[ErrorCode.ABORTED]],
  [/^Request timeout\.?$/i, ErrorMessages[ErrorCode.REQUEST_TIMEOUT]],
  [/^Network request failed\.?$/i, ErrorMessages[ErrorCode.NETWORK_ERROR]],
  [/^Network Error$/i, ErrorMessages[ErrorCode.NETWORK_ERROR]],
  [
    /^Unauthorized\. Please sign in again\.?$/i,
    ErrorMessages[ErrorCode.UNAUTHORIZED],
  ],
  [/^Permission denied\.?$/i, ErrorMessages[ErrorCode.FORBIDDEN]],
  [/^Invalid request parameters\.?$/i, ErrorMessages[ErrorCode.INVALID_PARAMS]],
  [
    /^Requested resource not found\.?$/i,
    ErrorMessages[ErrorCode.RESOURCE_NOT_FOUND],
  ],
  [
    /^Requested resource does not exist\.?$/i,
    ErrorMessages[ErrorCode.RESOURCE_NOT_FOUND],
  ],
  [/^Internal server error\.?$/i, "服务器内部错误，请稍后重试。"],
  [
    /^Duplicate request is in progress, please retry later\.?$/i,
    "已有相同请求正在处理中，请稍后重试。",
  ],
  [/^Failed to send request\.?$/i, "请求发送失败，请稍后重试。"],
  [/^Request failed\.?$/i, "请求失败，请稍后重试。"],
  [
    /^Request failed with status 413\.?$/i,
    "上传内容过大，请压缩文件或联系管理员调整上传限制。",
  ],
  [/^Request failed with status (\d+)\.?$/i, "请求失败，请稍后重试。"],
];

export const localizeErrorMessage = (
  message: string | null | undefined,
  fallback?: string,
) => {
  const normalized = message?.trim();
  if (!normalized) {
    return fallback;
  }

  const translated = messageTranslations.find(([pattern]) =>
    pattern.test(normalized),
  );
  return translated?.[1] ?? normalized;
};

export class AppError extends Error {
  public code: ErrorCode;
  public originalError?: unknown;

  constructor(code: ErrorCode, message?: string, originalError?: unknown) {
    super(message || ErrorMessages[code]);
    this.name = "AppError";
    this.code = code;
    this.originalError = originalError;
  }

  static from(
    error: unknown,
    defaultCode: ErrorCode = ErrorCode.UNKNOWN_ERROR,
  ): AppError {
    if (error instanceof AppError) {
      return error;
    }

    if (error instanceof Error) {
      if (error.name === "AbortError") {
        return new AppError(
          ErrorCode.ABORTED,
          ErrorMessages[ErrorCode.ABORTED],
          error,
        );
      }
      return new AppError(
        defaultCode,
        localizeErrorMessage(error.message, ErrorMessages[defaultCode]),
        error,
      );
    }

    if (typeof error === "string") {
      return new AppError(
        defaultCode,
        localizeErrorMessage(error, ErrorMessages[defaultCode]),
      );
    }

    return new AppError(defaultCode, undefined, error);
  }
}
