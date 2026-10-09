import { describe, expect, it } from "vitest";
import {
  LOG_STATUS_BADGE_CLASS,
  LOG_STATUSES,
  LOG_STATUS_TEXT_CLASS,
  getLogStatusBadgeClass,
  getLogStatusLabel,
  getLogStatusTextClass,
  type ChatLogStatus,
} from "./log-status";

/** 后端 models.ChatLogStatus* 的五个取值（2026-10-10 状态机分片 #1~#3 落地后的全集）。 */
const BACKEND_STATUSES: ChatLogStatus[] = ["processing", "success", "error", "canceled", "interrupted"];

describe("log-status", () => {
  it("清单与后端五态逐字对齐——后端新增状态而本表没跟上时这里变红", () => {
    expect([...LOG_STATUSES].sort()).toEqual([...BACKEND_STATUSES].sort());
  });

  it("每个状态都有非空中文文案，且不再直接渲染英文原文", () => {
    for (const status of BACKEND_STATUSES) {
      expect(getLogStatusLabel(status)).toBeTruthy();
      expect(getLogStatusLabel(status)).not.toBe(status);
    }
  });

  // 回归锚点：散落的 `status === "success" ? 绿 : 红` 会把 canceled / interrupted
  // 一律画成红色失败，那是冤枉上游的假结论。四态各自可辨是本表存在的理由。
  it("四个非 success 状态的文字色互不相同，不塌回同一个「红」", () => {
    const classes = BACKEND_STATUSES.map((status) => LOG_STATUS_TEXT_CLASS[status]);
    expect(new Set(classes).size).toBe(BACKEND_STATUSES.length);
  });

  it("四个非 success 状态的徽标色互不相同", () => {
    const classes = BACKEND_STATUSES.map((status) => LOG_STATUS_BADGE_CLASS[status]);
    expect(new Set(classes).size).toBe(BACKEND_STATUSES.length);
  });

  // 后端扩状态而前端还没跟上时，页面不能崩也不能空白：退回原文 + 中性色。
  it("未知状态降级为原文与中性色，不抛错也不返回 undefined", () => {
    expect(getLogStatusLabel("brand-new")).toBe("brand-new");
    expect(getLogStatusTextClass("brand-new")).toBe("text-muted-foreground");
    expect(getLogStatusBadgeClass("brand-new")).toBe("bg-muted text-muted-foreground");
  });
});
