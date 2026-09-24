import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  LOGIN_EXPIRY_NOTICE_STORAGE_KEY,
  LOGIN_EXPIRY_NOTICE_TEXT,
  consumeLoginExpiryNotice,
  hasLoginExpiryNotice,
  markLoginExpiredOnce,
} from "./login-expiry-notice";

beforeEach(() => {
  sessionStorage.clear();
  // markLoginExpiredOnce 有内存兜底标记（模块级状态）：不清会跨用例残留
  consumeLoginExpiryNotice();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("LOGIN_EXPIRY_NOTICE_TEXT", () => {
  it("展示文案钉死为字面量（对着常量比较的断言测不出常量本身被改坏）", () => {
    expect(LOGIN_EXPIRY_NOTICE_TEXT).toBe("登录已过期，请重新登录");
  });
});

describe("markLoginExpiredOnce", () => {
  it("首次标记返回 true，落下的值只含时间戳（响应体文本无从进入提示通道）", () => {
    expect(markLoginExpiredOnce()).toBe(true);
    const raw = sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY);
    expect(raw).toMatch(/^\d+$/);
  });

  it("去重窗口内不再触发（并发 401 只触发一次失效处置的判据）", () => {
    expect(markLoginExpiredOnce()).toBe(true);
    expect(markLoginExpiredOnce()).toBe(false);
    expect(markLoginExpiredOnce()).toBe(false);
  });

  it("窗口过后的陈旧标记不再抑制重新处置（处置被打断不把 401 出口永久缴械）", () => {
    const base = Date.now();
    expect(markLoginExpiredOnce()).toBe(true);
    // 标记值是写入时间戳（模块契约）：推过 10s 去重窗口后应可重新触发
    vi.spyOn(Date, "now").mockReturnValue(base + 11_000);
    expect(markLoginExpiredOnce()).toBe(true);
  });

  it("storage 写失败（隐私模式/配额满）时退内存兜底：本轮页面生命周期内去重与展示仍成立", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("QuotaExceededError");
    });

    expect(markLoginExpiredOnce()).toBe(true);
    expect(markLoginExpiredOnce()).toBe(false);
    expect(hasLoginExpiryNotice()).toBe(true);
  });
});

describe("hasLoginExpiryNotice / consumeLoginExpiryNotice", () => {
  it("有待展示提示时为 true；读后即焚——清除后为 false 且存储无残留（只展示一次）", () => {
    markLoginExpiredOnce();
    expect(hasLoginExpiryNotice()).toBe(true);

    consumeLoginExpiryNotice();

    expect(hasLoginExpiryNotice()).toBe(false);
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBeNull();
  });

  it("展示有效期 60s：陈旧标记不再展示（兜住「标记残留期间主动登出误显过期」的窗口）", () => {
    const base = Date.now();
    markLoginExpiredOnce();
    vi.spyOn(Date, "now").mockReturnValue(base + 61_000);
    expect(hasLoginExpiryNotice()).toBe(false);
  });

  it("没有标记时为 false，consume 不抛错", () => {
    expect(hasLoginExpiryNotice()).toBe(false);
    expect(() => consumeLoginExpiryNotice()).not.toThrow();
  });
});
