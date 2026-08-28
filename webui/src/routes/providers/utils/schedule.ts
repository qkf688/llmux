/**
 * 供应商调度配置（协议勾选 / 协议端点 / 凭据分组）——S0 原型形态。
 *
 * 原型期：这些字段随表单保存进 `Provider.Config` 的 `_schedule` 键（后端对 config 是
 * 自由 JSON，原样存取），供表单回填与列表徽标使用。S6 正式实现时改为独立 DTO 字段
 * （ProviderRequest 增加 protocols/endpoints/groups），本文件随之废弃。
 */

export interface ProviderScheduleEndpoint {
  protocol: string;
  /** 留空 = 继承 Provider 默认 Base URL */
  url: string;
  enabled: boolean;
}

export type ProviderScheduleSource = "inline" | "pool";

export interface ProviderScheduleGroup {
  name: string;
  /** 价格导向权重：便宜的分组权重高 → 加权随机命中概率大 */
  weight: number;
  /** 模型白名单，逗号分隔；空 = 不限 */
  models: string;
  /** 凭据来源二选一：内联 Key / 关联号池 */
  source: ProviderScheduleSource;
  inlineKeys: string;
  poolId: string;
}

export interface ProviderScheduleShape {
  protocols?: string[];
  endpoints?: ProviderScheduleEndpoint[];
  groups?: ProviderScheduleGroup[];
}

/** provider type → 出站协议（openai-res 对应 responses） */
export function protocolOfType(type: string): string {
  if (type === "anthropic") return "anthropic";
  if (type === "openai-res") return "responses";
  return "openai";
}

/** 从 Provider.Config 解析 `_schedule`，不存在或非法时返回 null */
export function parseScheduleFromConfig(config: string): ProviderScheduleShape | null {
  try {
    const parsed = JSON.parse(config);
    if (parsed && typeof parsed === "object" && parsed._schedule && typeof parsed._schedule === "object") {
      return parsed._schedule as ProviderScheduleShape;
    }
  } catch {
    // 非法 JSON：按无调度处理
  }
  return null;
}

/** 列表徽标用：端点 / 分组计数（无调度配置时均为 0） */
export function countSchedule(config: string): { endpoints: number; groups: number } {
  const schedule = parseScheduleFromConfig(config);
  return {
    endpoints: schedule?.endpoints?.length ?? 0,
    groups: schedule?.groups?.length ?? 0,
  };
}