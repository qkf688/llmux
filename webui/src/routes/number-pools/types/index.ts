/**
 * 号池页面类型（S0 原型）。
 * 仅承载 mock 数据形态；S6 接入后端后替换为 lib/api 的真实类型（PascalCase 字段）。
 */

/** 凭据状态（与设计定案第 5 节状态机对应） */
export type MockCredentialStatus = "active" | "cooldown" | "error" | "disabled";

export interface MockCredential {
  id: number;
  /** 凭据值（原型为演示值，正式实现为加密存储的 api key） */
  key: string;
  status: MockCredentialStatus;
  note?: string;
  /** 连续失败次数（error 判定依据） */
  failCount: number;
  /** 最近使用（展示用相对时间文本） */
  lastUsedAt?: string;
  totalRequests: number;
  totalErrors: number;
}

/** 号池被某个供应商分组引用（双向导航：号池详情 ↔ 分组） */
export interface MockPoolRefGroup {
  providerName: string;
  groupName: string;
}

export interface MockPool {
  id: number;
  name: string;
  note?: string;
  credentials: MockCredential[];
  refGroups: MockPoolRefGroup[];
}