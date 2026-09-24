import { beforeEach, describe, expect, it } from "vitest";
import {
  LOGIN_EXPIRY_NOTICE_STORAGE_KEY,
  LOGIN_EXPIRY_NOTICE_TEXT,
  consumeLoginExpiryNotice,
  setLoginExpiryNoticeOnce,
} from "./login-expiry-notice";

beforeEach(() => {
  sessionStorage.clear();
});

describe("setLoginExpiryNoticeOnce", () => {
  it("首次写入返回 true，落下的值是静态提示文案", () => {
    expect(setLoginExpiryNoticeOnce()).toBe(true);
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBe(LOGIN_EXPIRY_NOTICE_TEXT);
  });

  it("已设标记时不再写、返回 false（并发 401 只触发一次失效处置的去重判据）", () => {
    expect(setLoginExpiryNoticeOnce()).toBe(true);
    expect(setLoginExpiryNoticeOnce()).toBe(false);
    expect(setLoginExpiryNoticeOnce()).toBe(false);
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBe(LOGIN_EXPIRY_NOTICE_TEXT);
  });
});

describe("consumeLoginExpiryNotice", () => {
  it("读后即焚：取走提示并清除，二次读取为 null（只展示一次）", () => {
    setLoginExpiryNoticeOnce();

    expect(consumeLoginExpiryNotice()).toBe(LOGIN_EXPIRY_NOTICE_TEXT);
    expect(sessionStorage.getItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY)).toBeNull();
    expect(consumeLoginExpiryNotice()).toBeNull();
  });

  it("没有提示时返回 null，不抛错", () => {
    expect(consumeLoginExpiryNotice()).toBeNull();
  });
});
