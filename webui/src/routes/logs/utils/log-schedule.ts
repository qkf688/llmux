import type { ChatLog } from "@/lib/api";

/**
 * 调度列三态的单一派生源：desktop 表格 / mobile 列表 / 详情弹窗共用，
 * 防止视图双轨各自拼串后语义漂移（S0 原型时代 mobile 曾只显示一层、丢失另一层）。
 *
 * 三态语义：
 * - 完整态（S3-3 起新链路行）：两层均有值，正常展示；
 * - 部分态（边缘形态，如分组未命名）：缺的那层返回空串，由视图渲染占位符；
 * - 空态（S3-3 之前的存量行）：四字段全空，hasAny=false，视图应整体渲染占位符
 *   而非两层各挂一个「-」——调度四字段是建行时原子填充的，「逐层缺席」不是真实语义。
 */
export interface LogScheduleParts {
  /** false = 空态，视图整体占位 */
  hasAny: boolean;
  /** 端点层：协议 · URL；该层两字段都缺时为空串 */
  endpointLine: string;
  /** 凭据层：分组 · 凭据标识；该层两字段都缺时为空串 */
  credentialLine: string;
}

type LogScheduleFields = Pick<
  ChatLog,
  "endpoint_protocol" | "endpoint_url" | "key_group_name" | "credential_note"
>;

export function splitLogSchedule(log: LogScheduleFields): LogScheduleParts {
  const endpointLine = [log.endpoint_protocol, log.endpoint_url].filter(Boolean).join(" · ");
  const credentialLine = [log.key_group_name, log.credential_note].filter(Boolean).join(" · ");
  return {
    hasAny: endpointLine !== "" || credentialLine !== "",
    endpointLine,
    credentialLine,
  };
}
