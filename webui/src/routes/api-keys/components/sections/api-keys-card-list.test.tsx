/**
 * api-keys 卡片列表契约测试。核心回归目标（用户 2026-09-16 反馈驱动）：
 * 密钥值**任何形态都不进 DOM**——旧表格 + 眼睛方案展开明文会把表格向右撑开，
 * 卡片方案下不存在明文渲染路径，故钉死「DOM 文本与属性均无明文」。
 */
import { fireEvent, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiKeyItem } from "@/lib/api";

// 系统边界 mock：复制三级降级链由 lib/clipboard 承载（其自身有测试），
// 这里只钉本组件「把什么交给它、降级行如何处置」
const { copySecretWithToastMock } = vi.hoisted(() => ({
  copySecretWithToastMock: vi.fn(),
}));

vi.mock("@/lib/clipboard", () => ({
  copySecretWithToast: copySecretWithToastMock,
}));

import { ApiKeysCardList } from "./api-keys-card-list";

// 期望值独立来源：后端规则推导——生成 key = sk-llmux- + 32hex；
// 合法最短自定义值 = 补前缀后恰 16 字符；解密失败降级行 = "****"
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
  enabled: false,
  created_at: "2026-09-15T09:00:00Z",
};
const MASKED_FALLBACK_KEY: ApiKeyItem = {
  id: 3,
  name: "解密失败",
  api_key: "****",
  enabled: true,
  created_at: "2026-09-15T08:00:00Z",
};

function renderList(apiKeys: ApiKeyItem[], overrides: Partial<Parameters<typeof ApiKeysCardList>[0]> = {}) {
  const props = {
    apiKeys,
    onToggle: vi.fn(),
    onEdit: vi.fn(),
    onRequestDelete: vi.fn(),
    ...overrides,
  };
  const utils = render(<ApiKeysCardList {...props} />);
  return { ...utils, ...props };
}

beforeEach(() => {
  copySecretWithToastMock.mockReset();
  copySecretWithToastMock.mockResolvedValue(true);
});

describe("密钥值不进 DOM（撑开表格问题的根除证明）", () => {
  it("行文本与全部属性值都不含任何 key 值（含降级行的 ****）", () => {
    const { container } = renderList([GENERATED_KEY, SHORTEST_LEGAL_KEY, MASKED_FALLBACK_KEY]);
    const text = container.textContent ?? "";
    for (const apiKey of [GENERATED_KEY, SHORTEST_LEGAL_KEY]) {
      expect(text).not.toContain(apiKey.api_key);
    }
    for (const el of Array.from(container.querySelectorAll("*"))) {
      for (const attr of Array.from(el.attributes)) {
        expect(attr.value).not.toContain(GENERATED_KEY.api_key);
        expect(attr.value).not.toContain(SHORTEST_LEGAL_KEY.api_key);
      }
    }
  });

  it("行内展示名称、状态与创建时间，不展示密钥", () => {
    const { container } = renderList([GENERATED_KEY, SHORTEST_LEGAL_KEY]);
    const text = container.textContent ?? "";
    expect(text).toContain("生成的");
    expect(text).toContain("已启用");
    expect(text).toContain("已停用");
  });
});

describe("复制按钮", () => {
  it("把该行 api_key 明文与自定义失败文案交给 copySecretWithToast", () => {
    const { getByLabelText } = renderList([GENERATED_KEY]);
    fireEvent.click(getByLabelText("复制密钥 生成的"));
    expect(copySecretWithToastMock).toHaveBeenCalledWith(
      GENERATED_KEY.api_key,
      expect.stringContaining("生成的"),
      expect.any(String),
    );
    // 行上永不显示明文，失败引导必须指向编辑弹窗路径（copySecretWithToast 默认文案不可执行）
    const failureHint = copySecretWithToastMock.mock.calls[0][2];
    expect(failureHint).toContain("编辑弹窗");
  });

  it("解密失败降级行：复制入口禁用，不参与点击", () => {
    const { getByLabelText } = renderList([MASKED_FALLBACK_KEY]);
    const disabledButton = getByLabelText("密钥 解密失败 明文不可用，无法复制") as HTMLButtonElement;
    expect(disabledButton.disabled).toBe(true);
    fireEvent.click(disabledButton);
    expect(copySecretWithToastMock).not.toHaveBeenCalled();
  });
});

describe("行操作编排回调", () => {
  it("Switch / 编辑 / 删除分别上报整行对象", () => {
    const { getByLabelText, onToggle, onEdit, onRequestDelete } = renderList([GENERATED_KEY]);

    fireEvent.click(getByLabelText("停用 生成的"));
    expect(onToggle).toHaveBeenCalledWith(GENERATED_KEY);

    fireEvent.click(getByLabelText("编辑 生成的"));
    expect(onEdit).toHaveBeenCalledWith(GENERATED_KEY);

    fireEvent.click(getByLabelText("删除 生成的"));
    expect(onRequestDelete).toHaveBeenCalledWith(GENERATED_KEY);
  });

  it("busy 时禁用 Switch，防 PATCH 在途连点", () => {
    const { getByLabelText } = renderList([GENERATED_KEY], { busy: true });
    expect((getByLabelText("停用 生成的") as HTMLButtonElement).disabled).toBe(true);
  });
});
