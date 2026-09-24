import {
  getBrowserSessionStorage,
  readStorageString,
  removeStorageItem,
  writeStorageString,
} from "@/stores/core/storage";

/** 标记键：值 = 写入时间戳（毫秒）。标记兼两职——「失效处置已触发」的去重 + 「过期提示待展示」。 */
export const LOGIN_EXPIRY_NOTICE_STORAGE_KEY = "loginExpiryNotice";

/** 展示文案是静态常量（渲染侧取常量而非存储值）：结构性保证提示不携带任何后端/内部错误文本。 */
export const LOGIN_EXPIRY_NOTICE_TEXT = "登录已过期，请重新登录";

/** 处置去重窗口：同一波并发 401 只触发一次整套处置；窗口过后标记不再抑制重新处置
 *  （覆盖「处置中途被打断」的残余窗口：标记先写、跳转未成，不能把 401 出口永久缴械）。 */
const HANDLING_DEDUPE_WINDOW_MS = 10_000;

/** 展示有效期：标记写下后需撑过整页跳转 + 登录页加载（慢网络也够用）；
 *  超时的陈旧标记不再展示（主动登出后看到「登录已过期」的误导窗口被它兜住）。 */
const NOTICE_DISPLAY_WINDOW_MS = 60_000;

/** sessionStorage 写失败（隐私模式/配额满）时的页面内兜底：去重与展示退回内存标记，
 *  保证「只触发一次」在本轮页面生命周期内仍成立（跳转刷新后标记已无从谈起）。 */
let memoryMark: number | null = null;

/** 读标记时间戳：storage 优先，读不到退内存；值损坏（非数字）视为无标记。 */
function readMarkAt(): number | null {
  const raw = readStorageString(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, getBrowserSessionStorage());
  const parsed = raw === null ? memoryMark : Number(raw);
  return parsed !== null && Number.isFinite(parsed) ? parsed : null;
}

/**
 * 标记「登录过期处置已触发」（仅首次返回 true），返回本次是否由本调用触发。
 *
 * 调用方据此决定要不要执行清会话 + 跳登录页整套动作——并发多请求同时撞 401 时，
 * 只有首个调用方触发（去重窗口内的既有标记视为已处置），其余只抛错终止调用方。
 * 仅由「经鉴权通道收到 401」的失效出口调用；未登录直入（发起时本就无凭据）不标记，
 * 直接静默跳登录页（那是「未登录」不是「登录过期」，首访用户看到过期文案语义不符）。
 */
export function markLoginExpiredOnce(): boolean {
  const at = readMarkAt();
  if (at !== null && Date.now() - at < HANDLING_DEDUPE_WINDOW_MS) {
    return false;
  }
  const now = Date.now();
  memoryMark = now;
  writeStorageString(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, String(now), getBrowserSessionStorage());
  return true;
}

/** 是否有待展示的过期提示（非破坏性读取，渲染期可安全调用）；陈旧超时视为无。 */
export function hasLoginExpiryNotice(): boolean {
  const at = readMarkAt();
  return at !== null && Date.now() - at < NOTICE_DISPLAY_WINDOW_MS;
}

/** 清除过期标记（读后即焚，提示只展示一次）；登录页挂载时调用。 */
export function consumeLoginExpiryNotice(): void {
  memoryMark = null;
  removeStorageItem(LOGIN_EXPIRY_NOTICE_STORAGE_KEY, getBrowserSessionStorage());
}
