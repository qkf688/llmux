/**
 * 剪贴板写入（跨域共享单源：providers 模型名复制、号池 / 供应商内联 key 复制均走此处）。
 *
 * 降级链：安全上下文用 Clipboard API → 非安全上下文（http://<ip>:7070 这类）直接走
 * 同步 execCommand（保住点击手势有效期）→ 再失败弹手动复制 prompt。每级都回传 method。
 *
 * 两个带提示的入口按内容敏感度二选一：普通文本用 `copyTextWithToast`（保留 prompt 兜底），
 * 明文 key 等机密**必须**用 `copySecretWithToast`（禁 prompt——它会把机密整条弹上屏）。
 */
import { toast } from "sonner";

export type CopyTextMethod = "clipboard_api" | "exec_command" | "manual_prompt" | "none";

export type CopyTextResult = {
  ok: boolean;
  method: CopyTextMethod;
};

export function fallbackCopyViaExecCommand(text: string): boolean {
  if (typeof window === "undefined" || typeof document === "undefined") return false;

  const textArea = document.createElement("textarea");
  textArea.value = text;

  // Use readonly to avoid triggering mobile keyboards, but still allow selection.
  textArea.setAttribute("readonly", "");

  // Keep it off-screen and non-interactive.
  textArea.style.position = "fixed";
  textArea.style.top = "0";
  textArea.style.left = "-9999px";
  textArea.style.opacity = "0";
  textArea.style.pointerEvents = "none";

  document.body.appendChild(textArea);

  // Save the user's current page selection so we can restore it after copying.
  // Clearing the transient selection unconditionally would destroy their selection.
  const userSelection = window.getSelection?.();
  const savedRanges: Range[] = [];
  if (userSelection && userSelection.rangeCount > 0) {
    for (let i = 0; i < userSelection.rangeCount; i++) {
      savedRanges.push(userSelection.getRangeAt(i).cloneRange());
    }
  }

  let copied = false;
  try {
    // Avoid scroll jumps where supported.
    try {
      textArea.focus({ preventScroll: true });
    } catch {
      textArea.focus();
    }

    textArea.select();
    textArea.setSelectionRange(0, textArea.value.length);

    copied = document.execCommand("copy");
  } catch {
    copied = false;
  } finally {
    if (textArea.parentNode) {
      textArea.parentNode.removeChild(textArea);
    }

    // Restore the user's original selection instead of clearing it.
    try {
      const selection = window.getSelection();
      if (selection) {
        selection.removeAllRanges();
        for (const range of savedRanges) {
          selection.addRange(range);
        }
      }
    } catch {
      // ignore
    }
  }

  return copied;
}

export async function copyTextDetailed(
  text: string,
  opts?: { showManualPrompt?: boolean }
): Promise<CopyTextResult> {
  const showManualPrompt = opts?.showManualPrompt ?? true;

  if (typeof window === "undefined" || typeof document === "undefined") {
    return { ok: false, method: "none" };
  }

  // Non-secure contexts (e.g. http://<ip>:7070) block the Clipboard API.
  // Use the synchronous execCommand fallback immediately so the user gesture
  // from the click event is still active.
  // Compare against `=== false` so that browsers without `isSecureContext`
  // support (where the property is `undefined`) still try the Clipboard API
  // below instead of being treated as non-secure.
  if (window.isSecureContext === false) {
    if (fallbackCopyViaExecCommand(text)) {
      return { ok: true, method: "exec_command" };
    }

    return showManualPrompt ? manualPromptResult(text) : { ok: false, method: "none" };
  }

  // Secure context (or unknown): prefer the modern Clipboard API.
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return { ok: true, method: "clipboard_api" };
    }
  } catch {
    // Clipboard API failed, fall back below.
  }

  if (fallbackCopyViaExecCommand(text)) {
    return { ok: true, method: "exec_command" };
  }

  return showManualPrompt ? manualPromptResult(text) : { ok: false, method: "none" };
}

function manualPromptResult(text: string): CopyTextResult {
  try {
    const promptResult = window.prompt("浏览器限制：无法自动复制，请手动复制以下内容：", text);
    if (promptResult === null) {
      return { ok: false, method: "none" };
    }
    return { ok: false, method: "manual_prompt" };
  } catch {
    return { ok: false, method: "none" };
  }
}

/**
 * 复制普通文本并把结果翻成用户提示：成功 / 浏览器限制已弹手动窗口 / 彻底失败，三分支文案在此单源。
 *
 * successMessage 由调用方给出（含被复制内容的标识）。复制机密值请用 copySecretWithToast。
 */
export async function copyTextWithToast(text: string, successMessage: string): Promise<boolean> {
  return reportCopyResult(
    await copyTextDetailed(text, { showManualPrompt: true }),
    successMessage,
    "复制失败：当前环境不支持自动复制，请手动复制（建议使用 HTTPS/域名访问）。",
  );
}

/**
 * 复制**机密值**（明文 key 等）。与 copyTextWithToast 的唯一区别是禁用 prompt 兜底：
 * `window.prompt` 的默认值参数会把机密整条弹上屏——模态、可被肩窥/录屏捕获、且不受前端
 * 任何生命周期守卫管辖。对号池「未显示直接复制」这条路径，等于在用户从未点「显示」时
 * 扩大了明文可见面，与「列表恒掩码、明文只在显式动作后可见」的服务端掩码模型矛盾。
 *
 * 失败时引导用户走「先显示、再手动选中复制」：那条路径的明文是用户显式动作换来的。
 */
export async function copySecretWithToast(secret: string, successMessage: string): Promise<boolean> {
  return reportCopyResult(
    await copyTextDetailed(secret, { showManualPrompt: false }),
    successMessage,
    "复制失败：浏览器限制了自动复制。请先点「显示」，再手动选中明文复制（建议使用 HTTPS/域名访问）。",
  );
}

function reportCopyResult(
  result: CopyTextResult,
  successMessage: string,
  failureMessage: string,
): boolean {
  if (result.ok) {
    // 成功提示只回显调用方给的标识（掩码 / 行号）：toast 文案会落 DOM，复述机密等于绕过服务端掩码
    toast.success(successMessage);
    return true;
  }

  if (result.method === "manual_prompt") {
    toast.info(
      "浏览器限制：通过 IP + HTTP 访问时通常无法自动复制，已弹出手动复制窗口；建议改用 HTTPS/域名访问。",
    );
    return false;
  }

  toast.error(failureMessage);
  return false;
}
