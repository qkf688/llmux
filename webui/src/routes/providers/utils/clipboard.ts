export type CopyTextMethod = "clipboard_api" | "exec_command" | "manual_prompt" | "none";

export type CopyTextResult = {
  ok: boolean;
  method: CopyTextMethod;
};

function fallbackCopyViaExecCommand(text: string): boolean {
  try {
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

    // Avoid scroll jumps where supported.
    try {
      textArea.focus({ preventScroll: true });
    } catch {
      textArea.focus();
    }
    textArea.select();
    textArea.setSelectionRange(0, textArea.value.length);

    const copied = document.execCommand("copy");
    document.body.removeChild(textArea);
    return copied;
  } catch {
    return false;
  }
}

export async function copyTextDetailed(text: string, opts?: { showManualPrompt?: boolean }): Promise<CopyTextResult> {
  const showManualPrompt = opts?.showManualPrompt ?? true;

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return { ok: true, method: "clipboard_api" };
    }
  } catch {
    // Clipboard API failed, fallback below.
  }

  if (fallbackCopyViaExecCommand(text)) {
    return { ok: true, method: "exec_command" };
  }

  if (showManualPrompt) {
    try {
      // Last resort: let user copy manually (common on http://<ip> where clipboard APIs are blocked).
      window.prompt("浏览器限制：无法自动复制，请手动复制以下内容：", text);
      return { ok: false, method: "manual_prompt" };
    } catch {
      // ignore
    }
  }

  return { ok: false, method: "none" };
}

export async function copyText(text: string): Promise<boolean> {
  const result = await copyTextDetailed(text);
  return result.ok;
}
