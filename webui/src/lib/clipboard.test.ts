import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import {
  copySecretWithToast,
  copyTextDetailed,
  copyTextWithToast,
  fallbackCopyViaExecCommand,
} from "./clipboard";

// copyTextWithToast 的三分支提示由 sonner 承载，mock 掉以便断言「哪种失败给了哪级提示」
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
}));

// Helpers to stub browser-only globals that jsdom does not fully implement.

const setSecureContext = (value: boolean | undefined): void => {
  Object.defineProperty(window, "isSecureContext", {
    value,
    configurable: true,
    writable: true,
  });
};

const setClipboardWriteText = (impl?: (text: string) => Promise<void>): void => {
  const writeText = impl ?? vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
};

// jsdom does not implement document.execCommand, so we inject a mock function
// instead of using vi.spyOn (which requires the property to already exist).
const setExecCommand = (returnValue: boolean): void => {
  Object.defineProperty(document, "execCommand", {
    value: vi.fn().mockReturnValue(returnValue),
    configurable: true,
    writable: true,
  });
};

const setPrompt = (returnValue: string | null): void => {
  Object.defineProperty(window, "prompt", {
    value: vi.fn().mockReturnValue(returnValue),
    configurable: true,
    writable: true,
  });
};

describe("copyTextDetailed", () => {
  beforeEach(() => {
    // Default to a secure context with a working Clipboard API.
    setSecureContext(true);
    setClipboardWriteText();
    setExecCommand(true);
    setPrompt(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses Clipboard API in a secure context", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboardWriteText(writeText);

    const result = await copyTextDetailed("hello");

    expect(writeText).toHaveBeenCalledWith("hello");
    expect(result).toEqual({ ok: true, method: "clipboard_api" });
  });

  it("falls back to execCommand when Clipboard API throws", async () => {
    setClipboardWriteText(() => Promise.reject(new Error("denied")));
    setExecCommand(true);

    const result = await copyTextDetailed("hello");

    expect(result).toEqual({ ok: true, method: "exec_command" });
  });

  it("falls back to execCommand immediately in a non-secure context", async () => {
    setSecureContext(false);
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboardWriteText(writeText);
    setExecCommand(true);

    const result = await copyTextDetailed("hello");

    // Clipboard API must NOT be used in a non-secure context.
    expect(writeText).not.toHaveBeenCalled();
    expect(result).toEqual({ ok: true, method: "exec_command" });
  });

  it("returns manual_prompt when user confirms the prompt in a non-secure context", async () => {
    setSecureContext(false);
    setExecCommand(false);
    setPrompt("hello");

    const result = await copyTextDetailed("hello");

    expect(result).toEqual({ ok: false, method: "manual_prompt" });
  });

  it("returns none when user cancels the prompt in a non-secure context", async () => {
    setSecureContext(false);
    setExecCommand(false);
    setPrompt(null); // user clicked Cancel

    const result = await copyTextDetailed("hello");

    expect(result).toEqual({ ok: false, method: "none" });
  });

  it("returns none when user cancels the prompt after Clipboard API + execCommand fail", async () => {
    setSecureContext(true);
    setClipboardWriteText(() => Promise.reject(new Error("denied")));
    setExecCommand(false);
    setPrompt(null);

    const result = await copyTextDetailed("hello");

    expect(result).toEqual({ ok: false, method: "none" });
  });

  // Regression for bug #1: browsers without isSecureContext support (undefined)
  // must NOT be treated as non-secure; they should still try the Clipboard API.
  it("tries Clipboard API when isSecureContext is undefined (legacy browser)", async () => {
    setSecureContext(undefined);
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboardWriteText(writeText);

    const result = await copyTextDetailed("hello");

    expect(writeText).toHaveBeenCalledWith("hello");
    expect(result).toEqual({ ok: true, method: "clipboard_api" });
  });

  it("skips the prompt when showManualPrompt is false", async () => {
    setSecureContext(false);
    setExecCommand(false);
    setPrompt("hello");
    const promptSpy = vi.mocked(window.prompt);

    const result = await copyTextDetailed("hello", { showManualPrompt: false });

    expect(promptSpy).not.toHaveBeenCalled();
    expect(result).toEqual({ ok: false, method: "none" });
  });
});

describe("fallbackCopyViaExecCommand - user selection preservation", () => {
  const originalGetSelection = window.getSelection;

  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(window, "getSelection", {
      value: originalGetSelection,
      configurable: true,
      writable: true,
    });
  });

  // Regression for improvement #4: the fallback must restore the user's
  // original page selection instead of destroying it.
  it("restores the user's original selection after copying", () => {
    setExecCommand(true);

    // Build a fake range with a cloneRange that returns itself.
    const fakeRange = { cloneNode: vi.fn() } as unknown as Range;
    (fakeRange as unknown as { cloneRange: () => Range }).cloneRange = vi.fn().mockReturnValue(fakeRange);
    const addRange = vi.fn();
    const removeAllRanges = vi.fn();
    const getRangeAt = vi.fn().mockReturnValue(fakeRange);

    Object.defineProperty(window, "getSelection", {
      value: () => ({ rangeCount: 1, getRangeAt, removeAllRanges, addRange }),
      configurable: true,
      writable: true,
    });

    fallbackCopyViaExecCommand("hello");

    // After copy, removeAllRanges + addRange should restore the saved range.
    expect(removeAllRanges).toHaveBeenCalled();
    expect(addRange).toHaveBeenCalledWith(fakeRange);
  });
});

describe("fallbackCopyViaExecCommand - environment guard", () => {
  // Regression for improvement #3: the function must guard its own
  // window/document existence instead of relying on the caller.
  it("returns false when document is undefined", () => {
    const originalDocument = globalThis.document;
    Object.defineProperty(globalThis, "document", { value: undefined, configurable: true });

    const result = fallbackCopyViaExecCommand("hello");

    expect(result).toBe(false);

    Object.defineProperty(globalThis, "document", { value: originalDocument, configurable: true });
  });
});

describe("copyTextWithToast", () => {
  beforeEach(() => {
    setSecureContext(true);
    setClipboardWriteText();
    setExecCommand(true);
    setPrompt(null);
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.info).mockClear();
    vi.mocked(toast.error).mockClear();
  });

  it("reports success with the caller-provided label, never the copied text", async () => {
    const ok = await copyTextWithToast("sk-plaintext-value", "已复制 key: sk-****alue");

    expect(ok).toBe(true);
    // 成功提示只回显调用方给的标识（掩码）：toast 文案会落 DOM，复述明文等于绕过服务端掩码
    expect(vi.mocked(toast.success)).toHaveBeenCalledWith("已复制 key: sk-****alue");
    expect(vi.mocked(toast.info)).not.toHaveBeenCalled();
    expect(vi.mocked(toast.error)).not.toHaveBeenCalled();
  });

  it("says a manual copy window opened when the browser blocks automation", async () => {
    setSecureContext(false);
    setExecCommand(false);
    setPrompt("hello");

    const ok = await copyTextWithToast("hello", "已复制");

    expect(ok).toBe(false);
    expect(vi.mocked(toast.info)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.error)).not.toHaveBeenCalled();
  });

  it("reports failure when every copy path is unavailable", async () => {
    setSecureContext(false);
    setExecCommand(false);
    setPrompt(null);

    const ok = await copyTextWithToast("hello", "已复制");

    expect(ok).toBe(false);
    expect(vi.mocked(toast.error)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.info)).not.toHaveBeenCalled();
  });
});

describe("copySecretWithToast", () => {
  const SECRET = "sk-plaintext-value-ab12";
  const LABEL = "已复制 key: sk-****ab12";

  beforeEach(() => {
    // 非安全上下文 + execCommand 失败 = 普通复制会落到 prompt 兜底的那条路径，
    // 也正是本项目主部署形态（http://<ip>:7070）下机密复制最容易踩到的分支
    setSecureContext(false);
    setClipboardWriteText();
    setExecCommand(false);
    setPrompt(SECRET);
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.info).mockClear();
    vi.mocked(toast.error).mockClear();
  });

  it("never falls back to window.prompt, whose default value would put the secret on screen", async () => {
    const ok = await copySecretWithToast(SECRET, LABEL);

    expect(window.prompt).not.toHaveBeenCalled();
    expect(ok).toBe(false);
    expect(vi.mocked(toast.error)).toHaveBeenCalledTimes(1);
    // 失败提示引导用户改走「先显示、再手动选中复制」，且自身不得复述机密
    const message = String(vi.mocked(toast.error).mock.calls[0]?.[0]);
    expect(message).toContain("显示");
    expect(message).not.toContain(SECRET);
  });

  it("reports success with the caller-provided label, never the secret", async () => {
    setSecureContext(true);
    setExecCommand(true);

    const ok = await copySecretWithToast(SECRET, LABEL);

    expect(ok).toBe(true);
    expect(vi.mocked(toast.success)).toHaveBeenCalledWith(LABEL);
    expect(String(vi.mocked(toast.success).mock.calls[0]?.[0])).not.toContain(SECRET);
  });

  it("caller-supplied failure hint replaces the default 先显示 guidance", async () => {
    // 明文永不渲染的页面（api-keys 卡片列表）传自定义引导；不传时维持默认文案（上一组用例钉住）
    const ok = await copySecretWithToast(SECRET, LABEL, "复制失败：请改用 HTTPS 访问后重试。");

    expect(ok).toBe(false);
    expect(String(vi.mocked(toast.error).mock.calls[0]?.[0])).toBe(
      "复制失败：请改用 HTTPS 访问后重试。",
    );
  });
});
