/**
 * 编辑弹窗密钥区契约测试（grilling 共识 2026-09-16）：
 * IP+HTTP 环境剪贴板不可用时，明文查看走编辑弹窗兜底——
 * 默认掩码、显式点击才展开（明文不进 DOM 直到显式动作）、可收起、
 * 关窗重开回落掩码；解密失败降级行只给不可用提示。
 *
 * Radix Dialog 经 Portal 挂到 document.body，断言一律用 screen / body，
 * 不用 render() 返回的 container（里面只有空的 root）。
 */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiKeyItem } from "@/lib/api";

const { copySecretWithToastMock } = vi.hoisted(() => ({
  copySecretWithToastMock: vi.fn(),
}));

vi.mock("@/lib/clipboard", () => ({
  copySecretWithToast: copySecretWithToastMock,
}));

import { ApiKeyFormDialog } from "./api-key-form-dialog";
import { TooltipProvider } from "@/components/ui/tooltip";

const EDITING_KEY: ApiKeyItem = {
  id: 1,
  name: "生产用",
  api_key: "sk-llmux-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  enabled: true,
  created_at: "2026-09-15T10:00:00Z",
};
const MASKED_FALLBACK_KEY: ApiKeyItem = {
  ...EDITING_KEY,
  id: 2,
  name: "解密失败",
  api_key: "****",
};

function bodyHasSecret(plain: string): boolean {
  if (document.body.textContent?.includes(plain)) return true;
  for (const el of Array.from(document.body.querySelectorAll("*"))) {
    for (const attr of Array.from(el.attributes)) {
      if (attr.value.includes(plain)) return true;
    }
  }
  return false;
}

function renderDialog(apiKey: ApiKeyItem | null) {
  const props = {
    open: true,
    onOpenChange: vi.fn(),
    apiKey,
    onSaved: vi.fn(),
  };
  // App 根挂 TooltipProvider，创建模式的 ⓘ 提示依赖它——测试里同构包一层
  const utils = render(
    <TooltipProvider>
      <ApiKeyFormDialog {...props} />
    </TooltipProvider>,
  );
  return { ...utils, ...props };
}

beforeEach(() => {
  copySecretWithToastMock.mockReset();
  copySecretWithToastMock.mockResolvedValue(true);
});

describe("编辑模式默认掩码", () => {
  it("未点显示前，DOM 文本与属性都不含完整明文，密钥区呈掩码形态", () => {
    renderDialog(EDITING_KEY);
    expect(screen.getByText("sk-llmux-0a1…e8f9")).toBeTruthy();
    expect(bodyHasSecret(EDITING_KEY.api_key)).toBe(false);
  });

  it("创建模式没有密钥区（还没有值可显示）", () => {
    renderDialog(null);
    expect(screen.queryByLabelText("显示明文")).toBeNull();
    expect(bodyHasSecret(EDITING_KEY.api_key)).toBe(false);
  });

  it("创建模式：内嵌前缀输入框剥重复前缀，提交拼回完整值；留空不提交 value", async () => {
    const { onSaved } = renderDialog(null);
    fireEvent.change(screen.getByLabelText("名称"), { target: { value: "my" } });

    // Label 内嵌了 ⓘ 触发按钮（其文本会进可及名计算），限定 selector 锁定输入框本体
    const valueInput = screen.getByLabelText(/自定义密钥/, {
      selector: "input",
    }) as HTMLInputElement;
    expect(valueInput.placeholder).toBe("留空则自动生成");

    // 粘贴含前缀的完整值：框内只留后缀（前缀是展示件，防视觉双前缀）
    fireEvent.change(valueInput, { target: { value: "sk-llmux-dead0000beef" } });
    expect(valueInput.value).toBe("dead0000beef");

    // zodResolver 异步校验：handleSubmit 的回调在微任务之后才触发，必须 waitFor
    fireEvent.click(screen.getByText("创建"));
    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({
        name: "my",
        value: "sk-llmux-dead0000beef",
      }),
    );

    // 清空后缀 = 留空 → value 不带（后端自动生成）
    fireEvent.change(valueInput, { target: { value: "" } });
    fireEvent.click(screen.getByText("创建"));
    await waitFor(() =>
      expect(onSaved).toHaveBeenLastCalledWith({ name: "my", value: undefined }),
    );
  });
});

describe("显示 / 收起明文", () => {
  it("点「显示明文」后完整明文上屏且带 translate=no；点收起回掩码", () => {
    renderDialog(EDITING_KEY);

    fireEvent.click(screen.getByLabelText("显示明文"));
    const plainEl = screen.getByText(EDITING_KEY.api_key);
    expect(plainEl.getAttribute("translate")).toBe("no");

    fireEvent.click(screen.getByLabelText("隐藏明文"));
    expect(screen.queryByLabelText("显示明文")).toBeTruthy();
    expect(bodyHasSecret(EDITING_KEY.api_key)).toBe(false);
  });

  it("弹窗关后再开，明文态回落掩码（显示是单次显式动作换来的）", () => {
    const { rerender } = renderDialog(EDITING_KEY);
    fireEvent.click(screen.getByLabelText("显示明文"));
    expect(bodyHasSecret(EDITING_KEY.api_key)).toBe(true);

    rerender(<ApiKeyFormDialog open={false} onOpenChange={vi.fn()} apiKey={EDITING_KEY} onSaved={vi.fn()} />);
    rerender(<ApiKeyFormDialog open onOpenChange={vi.fn()} apiKey={EDITING_KEY} onSaved={vi.fn()} />);

    expect(bodyHasSecret(EDITING_KEY.api_key)).toBe(false);
  });
});

describe("弹窗内复制与降级行", () => {
  it("复制把明文交给 copySecretWithToast，失败引导指向弹窗内显示明文（自包含，不指向别处）", () => {
    renderDialog(EDITING_KEY);
    fireEvent.click(screen.getByLabelText(`复制密钥 ${EDITING_KEY.name}`));
    expect(copySecretWithToastMock).toHaveBeenCalledWith(
      EDITING_KEY.api_key,
      expect.stringContaining(EDITING_KEY.name),
      expect.any(String),
    );
    const failureHint = String(copySecretWithToastMock.mock.calls[0]?.[2]);
    expect(failureHint).toContain("显示明文");
  });

  it("解密失败降级行：只给不可用提示，无显示/复制入口", () => {
    renderDialog(MASKED_FALLBACK_KEY);
    expect(document.body.textContent ?? "").toContain("明文不可用");
    expect(screen.queryByLabelText("显示明文")).toBeNull();
    expect(screen.queryByLabelText(`复制密钥 ${MASKED_FALLBACK_KEY.name}`)).toBeNull();
  });
});

describe("弹窗标题语义", () => {
  it("编辑态标题为「编辑 API Key」（改名 + 查看 + 复制三合一入口）", () => {
    renderDialog(EDITING_KEY);
    expect(screen.getByText("编辑 API Key")).toBeTruthy();
  });
});
