import {
  getBrowserSessionStorage,
  readStorageString,
  removeStorageItem,
  writeStorageString,
} from "@/stores/core/storage";

/** 提示与去重标记同一个键：值存在 = 「失效处置已触发、登录页有待展示的提示」。 */
export const LOGIN_EXPIRY_NOTICE_STORAGE_KEY = "loginExpiryNotice";

/** 展示文案是静态常量：提示只承载「登录已过期」这一事实，禁携带后端/内部错误文本。 */
export const LOGIN_EXPIRY_NOTICE_TEXT = "登录已过期，请重新登录";

/**
 * 写入一次性「登录已过期」提示（仅首次），返回本次是否写入。
 *
 * 返回值兼作失效处置的去重标记：并发多请求同时撞 401 时，只有首个调用方执行
 * 清会话 + 跳登录页整套动作（已设标记不再跳/不再写），其余只抛错终止调用方。
 * 标记放 sessionStorage 而非内存——跳转是整页刷新，内存状态撑不过跳转；
 * 登录页挂载时经 consumeLoginExpiryNotice 读后即焚，标记随提示一并消失。
 */
export function setLoginExpiryNoticeOnce(): boolean {
  const storage = getBrowserSessionStorage();
  if (readStorageString(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, storage) !== null) {
    return false;
  }
  writeStorageString(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, LOGIN_EXPIRY_NOTICE_TEXT, storage);
  return true;
}

/** 取走一次性提示并清除（读后即焚，只展示一次）；没有则返回 null。 */
export function consumeLoginExpiryNotice(): string | null {
  const storage = getBrowserSessionStorage();
  const text = readStorageString(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, storage);
  if (text === null) {
    return null;
  }
  removeStorageItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, storage);
  return text;
}
