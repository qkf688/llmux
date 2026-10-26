"use client";

export type CompactCount = {
  raw: number;
  value: string;
  unit: string;
};

export function formatCompactCount(input?: number): CompactCount {
  const raw = typeof input === "number" ? input : 0;
  const abs = Math.abs(raw);

  if (abs >= 1_000_000_000) {
    return { raw, value: (raw / 1_000_000_000).toFixed(2), unit: "B" };
  }
  if (abs >= 1_000_000) {
    return { raw, value: (raw / 1_000_000).toFixed(2), unit: "M" };
  }
  if (abs >= 1_000) {
    return { raw, value: (raw / 1_000).toFixed(2), unit: "K" };
  }

  return { raw, value: Math.round(raw).toLocaleString(), unit: "" };
}

export function formatPercent(value?: number, digits = 1): string {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "-";
  }
  return `${(value * 100).toFixed(digits)}%`;
}

/** 首包等耗时字段的入参口径是**毫秒**：后端 avg_response_time / ResponseTimeMs
 *  均为毫秒累计（见 repository/stats.go）。旧名 formatTimeNs 按纳秒处理，
 *  供应商排行把 4370ms 显示成 4.37 μs，差 6 个数量级。 */
export function formatTimeMs(milliseconds?: number): string {
  if (typeof milliseconds !== "number" || Number.isNaN(milliseconds)) {
    return "-";
  }

  if (milliseconds < 1) {
    return `${(milliseconds * 1000).toFixed(2)} μs`;
  }
  if (milliseconds < 1000) {
    return `${milliseconds.toFixed(2)} ms`;
  }
  return `${(milliseconds / 1000).toFixed(2)} s`;
}

/** ISO/RFC3339 时间串 → 本地可读格式。全站管理端单源，locale 固定 zh-CN
 *  （全站界面即中文），空值回 "-"。原 routes/logs、health-check-logs、
 *  model-sync-logs 的本地副本已于 2026-09-20 沉并收编，勿在页面层再写本地版 */
export function formatDateTime(value?: string): string {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN");
}

