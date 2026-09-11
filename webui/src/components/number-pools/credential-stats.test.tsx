import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatCredentialLastUsed } from "./credential-status";
import { CredentialStats } from "./credential-stats-cell";
import { render, screen } from "@testing-library/react";

const NOW = new Date("2026-09-12T12:00:00Z").getTime();

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  vi.useRealTimers();
});

const isoSecondsAgo = (sec: number) => new Date(NOW - sec * 1000).toISOString();

describe("formatCredentialLastUsed", () => {
  it("null/undefined/非法串 → 从未使用", () => {
    expect(formatCredentialLastUsed(null)).toBe("从未使用");
    expect(formatCredentialLastUsed(undefined)).toBe("从未使用");
    expect(formatCredentialLastUsed("not-a-date")).toBe("从未使用");
  });

  it("60 秒内 → 刚刚", () => {
    expect(formatCredentialLastUsed(isoSecondsAgo(30))).toBe("刚刚");
  });

  it("分/时/天桶无空格（gpt-load 式紧凑读数）", () => {
    expect(formatCredentialLastUsed(isoSecondsAgo(5 * 60))).toBe("5分钟前");
    expect(formatCredentialLastUsed(isoSecondsAgo(23 * 3600))).toBe("23小时前");
    expect(formatCredentialLastUsed(isoSecondsAgo(3 * 86400))).toBe("3天前");
  });

  it("超过 7 天回落到本地日期", () => {
    const iso = isoSecondsAgo(30 * 86400);
    expect(formatCredentialLastUsed(iso)).toBe(new Date(iso).toLocaleDateString());
  });
});

describe("CredentialStats", () => {
  it("渲染「请求 N · 失败 M · 相对时间」单行", () => {
    render(
      <CredentialStats
        totalRequests={623}
        totalErrors={0}
        lastUsedAt={isoSecondsAgo(23 * 3600)}
      />,
    );
    expect(screen.getByTestId("credential-stats").textContent).toBe(
      "请求 623 · 失败 0 · 23小时前",
    );
  });

  it("从未使用时显示占位文案", () => {
    render(<CredentialStats totalRequests={0} totalErrors={0} lastUsedAt={null} />);
    expect(screen.getByTestId("credential-stats").textContent).toBe(
      "请求 0 · 失败 0 · 从未使用",
    );
  });
});
