import service from "@/lib/request";
import { clearAuthToken, setAuthToken } from "@/lib/authToken";
import { AppError, ErrorCode } from "@/lib/errors";
import type {
  AuthPayloadDTO,
  ResultBoolean,
  ResultVoid,
  UserActualRespDTO,
  UserLoginReqDTO,
  UserRegisterReqDTO,
  UserRespDTO,
} from "@/types/auth";

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const toNumber = (value: unknown): number | undefined => {
  if (typeof value === "number") return Number.isNaN(value) ? undefined : value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isNaN(parsed) ? undefined : parsed;
  }
  return undefined;
};

const toString = (value: unknown): string | undefined => {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean")
    return String(value);
  return undefined;
};

const normalizeUser = (raw: unknown): UserRespDTO | null => {
  if (!isRecord(raw)) return null;

  const username = toString(raw.username) || "";
  if (!username) return null;

  return {
    id: toNumber(raw.id),
    username,
    realName: toString(raw.realName ?? raw.real_name),
    phone: toString(raw.phone),
    mail: toString(raw.mail),
    avatar: toString(raw.avatar),
    deletionTime: toNumber(raw.deletionTime ?? raw.deletion_time),
    createTime: toString(raw.createTime ?? raw.create_time),
    updateTime: toString(raw.updateTime ?? raw.update_time),
    delFlag: toNumber(raw.delFlag ?? raw.del_flag) as 0 | 1 | undefined,
  };
};

const extractUserFromAuthPayload = (payload: unknown): UserRespDTO | null => {
  const direct = normalizeUser(payload);
  if (direct) return direct;
  if (!isRecord(payload)) return null;
  return normalizeUser(payload.user) || normalizeUser(payload.currentUser);
};

const extractTokenFromAuthPayload = (payload: unknown): string | null => {
  if (!isRecord(payload)) return null;

  const directToken = toString(
    payload.token ?? payload.accessToken ?? payload.authToken,
  )?.trim();
  if (directToken) return directToken;

  const nestedData = payload.data;
  if (isRecord(nestedData)) {
    const nestedToken = toString(
      nestedData.token ?? nestedData.accessToken ?? nestedData.authToken,
    )?.trim();
    if (nestedToken) return nestedToken;
  }

  return null;
};

export const authService = {
  login: async (data: UserLoginReqDTO) => {
    const payload = await service.post<AuthPayloadDTO>(
      "/lingzhi/v1/users/login",
      data,
    );
    const token = extractTokenFromAuthPayload(payload);
    if (token) {
      setAuthToken(token);
    }

    const user = extractUserFromAuthPayload(payload);
    if (!user) {
      throw new Error("登录成功，但返回的用户信息缺失");
    }
    if (!token) {
      throw new Error("登录成功，但返回的登录凭证缺失");
    }
    return user;
  },

  register: (data: UserRegisterReqDTO) => {
    return service.post<ResultVoid>("/lingzhi/v1/users/register", data);
  },

  checkLogin: async () => {
    const payload = await service.get<AuthPayloadDTO>(
      "/lingzhi/v1/users/check-login",
    );
    const token = extractTokenFromAuthPayload(payload);
    if (token) {
      setAuthToken(token);
    }

    const user = extractUserFromAuthPayload(payload);
    if (!user) {
      throw new AppError(ErrorCode.UNAUTHORIZED, "用户未登录，请先登录");
    }
    return user;
  },

  logout: async () => {
    try {
      return await service.post<ResultVoid>("/lingzhi/v1/users/logout");
    } finally {
      clearAuthToken();
    }
  },

  getUser: (username: string) => {
    return service.get<UserRespDTO>(`/lingzhi/v1/users/${username}`);
  },

  getUserActual: (username: string) => {
    return service.get<UserActualRespDTO>(
      `/lingzhi/v1/users/actual/${username}`,
    );
  },

  hasUsername: (username: string) => {
    return service.get<ResultBoolean>("/lingzhi/v1/users/has-username", {
      params: { username },
    });
  },
};
