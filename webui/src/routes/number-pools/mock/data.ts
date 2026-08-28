/**
 * S0 原型数据源（项目首个 mock 先例）。
 * 仅开发预览用：S6 接入后端后删除本文件，替换为 TanStack Query 调用。
 * 状态分布采用确定性伪随机（与 id 挂钩），避免每次刷新页面数据跳变。
 */
import type { MockCredential, MockCredentialStatus, MockPool } from "../types";

const KEY_PREFIX = "sk-proto-";
const HEX_CHARS = "0123456789abcdef";

function randHex(length: number): string {
  let out = "";
  for (let i = 0; i < length; i++) {
    out += HEX_CHARS[Math.floor(Math.random() * HEX_CHARS.length)];
  }
  return out;
}

function makeKey(id: number): string {
  return `${KEY_PREFIX}${randHex(24)}${String(id).padStart(4, "0")}`;
}

/** 确定性状态分布：error/cooldown 只在池子够大时出现（演示健康概览的多样性） */
function pickStatus(id: number, total: number): MockCredentialStatus {
  const r = (id * 7919) % 100;
  if (total >= 20 && r < 5) return "error";
  if (total >= 20 && r < 11) return "cooldown";
  if (r < 15) return "disabled";
  return "active";
}

function makeCredential(id: number, total: number): MockCredential {
  const status = pickStatus(id, total);
  return {
    id,
    key: makeKey(id),
    status,
    note: status === "error" ? "鉴权失败连续超限" : status === "cooldown" ? "429 限流冷却中" : undefined,
    failCount: status === "error" ? 3 + (id % 9) : status === "cooldown" ? 1 + (id % 2) : 0,
    lastUsedAt: `${1 + (id % 60)} 分钟前`,
    totalRequests: 100 + ((id * 97) % 9000),
    totalErrors: status === "error" ? 5 + (id % 40) : id % 20,
  };
}

export const mockPools: MockPool[] = [
  {
    id: 1,
    name: "OpenAI 主池",
    note: "生产主密钥池（原型数据，120 keys 演示批量导入形态）",
    credentials: Array.from({ length: 120 }, (_, i) => makeCredential(i + 1, 120)),
    refGroups: [
      { providerName: "openai-prod", groupName: "默认组" },
      { providerName: "openai-prod", groupName: "低价组" },
    ],
  },
  {
    id: 2,
    name: "Anthropic 备用池",
    note: "备用 & 限流分流（原型数据）",
    credentials: Array.from({ length: 35 }, (_, i) => makeCredential(i + 1001, 35)),
    refGroups: [{ providerName: "anthropic-main", groupName: "默认组" }],
  },
  {
    id: 3,
    name: "低价分流池",
    note: "低价模型专用（原型数据）",
    credentials: Array.from({ length: 12 }, (_, i) => makeCredential(i + 2001, 12)),
    refGroups: [{ providerName: "deepseek-flash", groupName: "走量组" }],
  },
];
