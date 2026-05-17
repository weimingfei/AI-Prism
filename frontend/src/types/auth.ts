export interface UserLoginReqDTO {
  username?: string;
  password?: string;
}

export interface UserRegisterReqDTO {
  username?: string;
  password?: string;
  realName?: string;
  phone?: string;
  mail?: string;
}

// 后端用户表映射：t_user
export interface UserEntity {
  id?: number;
  username: string;
  password?: string | null;
  realName?: string | null;
  phone?: string | null;
  mail?: string | null;
  deletionTime?: number | null;
  createTime?: string | null;
  updateTime?: string | null;
  delFlag?: 0 | 1 | null;
}

// 前端安全 DTO，不包含密码字段。
export type UserRespDTO = Omit<UserEntity, "password"> & {
  avatar?: string | null;
};

export type UserActualRespDTO = UserRespDTO;
export type ResultVoid = null;
export type ResultBoolean = boolean;

// 登录和 check-login 的返回在兼容期可能被不同 key 包一层。
export type AuthPayloadDTO = {
  token?: string;
  user?: unknown;
  currentUser?: unknown;
  [key: string]: unknown;
};
