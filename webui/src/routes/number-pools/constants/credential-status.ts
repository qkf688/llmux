import type { MockCredentialStatus } from "../types";

/**
 * 凭据状态展示契约（label），与 mock 数据源解耦：
 * 随 S6 引入真实凭据状态机时此表继续演进（状态类型扩展时查询 types/index.ts 的联合类型）。
 */
export const CREDENTIAL_STATUS_LABEL: Record<MockCredentialStatus, string> = {
  active: "健康",
  cooldown: "冷却",
  error: "错误",
  disabled: "停用",
};