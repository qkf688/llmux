import { fireEvent, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiKeyItem } from "@/lib/api";

// 系统边界 mock：复制的三级降级链由 lib/clipboard 承载（其自身有测试），
// 这里只钉本组件「把什么交给它、显示态如何变化」
const { copySecretWithToastMock } = vi.hoisted(() => ({
  copySecretWithToastMock: vi.fn(),
}));

vi.mock("@/lib/clipboard", () => ({
  copySecretWithToast: copySecretWithToastMock,
}));

import { ApiKeysKeyCell } from "./api-keys-key-cell";

// 期望值独立来源：后端规则推导——生成 key = sk-llmux- + 32hex（41 字符）；
// 合法最短自定义值 = 补前缀后恰 16 字符；降级行 = "****"
const GENERATED_KEY: ApiKeyItem = {
  id: 1,
  name: "生成的",
  api_key: "sk-llmux-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  enabled: true,
  created_at: "2026-09-15T10:00:00Z",
};
const SHORTEST_LEGAL_KEY: ApiKeyItem = {
  id: 2,
  name: "最短合法值",
  api_key: "sk-llmux-1234567",
  enabled: true,
  created_at: "2026-09-15T10:00:00Z",
};
const MASKED_FALLBACK_KEY: ApiKeyItem = {
  id: 3,
  name: "解密失败",
  api_key: "****",
  enabled: true,
  created_at: "2026-09-15T10:00:00Z",
};

function renderCell(apiKey: ApiKeyItem, overrides: Partial<Parameters<typeof ApiKeysKeyCell>[0]> = {}) {
  const onToggleReveal = vi.fn();
  const utils = render(
    <ApiKeysKeyCell
      apiKey={apiKey}
      revealed={false}
      onToggleReveal={onToggleReveal}
      {...overrides}
    />,
  );
  return { ...utils, onToggleReveal };
}

beforeEach(() => {
  copySecretWithToastMock.mockReset();
  copySecretWithToastMock.mockResolvedValue(true);
});

describe("ApiKeysKeyCell 掩码态", () => {
  it("真实 key（含 16 字符最短合法值）：DOM 文本与 title 均不含完整明文", () => {
    for (const apiKey of [GENERATED_KEY, SHORTEST_LEGAL_KEY]) {
      const { container, unmount } = renderCell(apiKey);
      expect(container.textContent).not.toContain(apiKey.api_key);
      for (const el of container.querySelectorAll("[title]")) {
        expect(el.getAttribute("title")).not.toContain(apiKey.api_key);
      }
      unmount();
    }
  });

  it("解密失败降级行（api_key 即掩码符）原样显示，不存在截断分支", () => {
    const { container } = renderCell(MASKED_FALLBACK_KEY);
    expect(container.textContent).toContain("****");
  });

  it("长 key 掩码形态 = 前 12 …后 4", () => {
    const { container } = renderCell(GENERATED_KEY);
    expect(container.textContent).toContain("sk-llmux-0a1…e8f9");
  });

  it("眼睛按钮只上报 id，明文可见与否由上层控制", () => {
    const { getByLabelText, onToggleReveal } = renderCell(GENERATED_KEY);
    fireEvent.click(getByLabelText("显示密钥 生成的 的明文"));
    expect(onToggleReveal).toHaveBeenCalledWith(GENERATED_KEY.id);
  });
});

describe("ApiKeysKeyCell 明文态与复制", () => {
  it("revealed 时渲染完整明文且带 translate=no，明文态不设 title", () => {
    const { container, getByLabelText } = renderCell(GENERATED_KEY, { revealed: true });
    const text = container.querySelector("span");
    expect(text?.textContent).toBe(GENERATED_KEY.api_key);
    expect(text?.getAttribute("translate")).toBe("no");
    expect(text?.hasAttribute("title")).toBe(false);
    expect(getByLabelText("隐藏密钥 生成的")).toBeTruthy();
  });

  it("复制把该行 api_key 明文交给 copySecretWithToast，不自行切换显示态", () => {
    const { getByLabelText, onToggleReveal } = renderCell(GENERATED_KEY);
    fireEvent.click(getByLabelText("复制密钥 生成的"));
    expect(copySecretWithToastMock).toHaveBeenCalledWith(
      GENERATED_KEY.api_key,
      expect.stringContaining("生成的"),
    );
    expect(onToggleReveal).not.toHaveBeenCalled();
  });
});
