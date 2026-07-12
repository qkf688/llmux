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

export async function copyText(text: string): Promise<boolean> {
  const result = await copyTextDetailed(text);
  return result.ok;
}
