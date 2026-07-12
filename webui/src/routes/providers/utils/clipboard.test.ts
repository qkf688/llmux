import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { copyTextDetailed, fallbackCopyViaExecCommand } from "./clipboard";

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
