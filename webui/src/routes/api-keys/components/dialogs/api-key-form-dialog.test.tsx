/**
 * 编辑弹窗密钥区契约测试（grilling 共识 2026-09-16）：
 * IP+HTTP 环境剪贴板不可用时，明文查看走编辑弹窗兜底——
 * 默认掩码、显式点击才展开（明文不进 DOM 直到显式动作）、可收起、
 * 关窗重开回落掩码；解密失败降级行只给不可用提示。
 * 「明文不进 DOM」的「明文」特指 api_key 密钥值；#6 headers 编辑区的值属
 * 展示配置（非密钥语义），无条件渲染进 Input，不受 reveal 门控（Rulings 语境）。
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
    // #6 契约：弹窗恒提交 headers 键（表单即真相；空区={}，Rulings 5）
    fireEvent.click(screen.getByText("创建"));
    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({
        name: "my",
        value: "sk-llmux-dead0000beef",
        headers: {},
      }),
    );

    // 清空后缀 = 留空 → value 不带（后端自动生成）
    fireEvent.change(valueInput, { target: { value: "" } });
    fireEvent.click(screen.getByText("创建"));
    await waitFor(() =>
      expect(onSaved).toHaveBeenLastCalledWith({ name: "my", value: undefined, headers: {} }),
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

// —— #6 key 级自定义出站头编辑区（方案 §2 切片 B；Rulings 1/2/5） ——————————

const KEYED_HEADERS_ITEM: ApiKeyItem = {
  ...EDITING_KEY,
  id: 4,
  headers: { "User-Agent": "claude-cli/1.0.70", "X-App": "cli" },
};

describe("#6 headers 编辑区", () => {
  it("创建与编辑两模式都渲染编辑区与诊断边界提示", () => {
    renderDialog(null);
    expect(screen.getByText("自定义出站头")).toBeTruthy();
    expect(document.body.textContent ?? "").toContain("不参与诊断");

    render(<TooltipProvider><ApiKeyFormDialog open onOpenChange={vi.fn()} apiKey={KEYED_HEADERS_ITEM} onSaved={vi.fn()} /></TooltipProvider>);
    expect(screen.getAllByText("自定义出站头").length).toBeGreaterThan(1);
  });

  it("编辑模式回显既有 headers（round-trip 的读侧）", () => {
    renderDialog(KEYED_HEADERS_ITEM);
    const keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    const values = keyInputs.map((input) => input.value);
    expect(values).toContain("User-Agent");
    expect(values).toContain("X-App");
    const valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    expect(valueInputs.map((input) => input.value)).toContain("claude-cli/1.0.70");
  });

  it("手动行：空名行跳过、同名后者赢、提交得到 Record", async () => {
    const { onSaved } = renderDialog(null);
    fireEvent.change(screen.getByLabelText("名称"), { target: { value: "k" } });

    const add = screen.getByText("添加出站头");
    fireEvent.click(add);
    fireEvent.click(add);
    fireEvent.click(add);
    const keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    const valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    // 第 1 行留空（应被跳过）；第 2/3 行同名不同大小写（后者赢，键名取后者）
    fireEvent.change(keyInputs[1], { target: { value: "X-Foo" } });
    fireEvent.change(valueInputs[1], { target: { value: "first" } });
    fireEvent.change(keyInputs[2], { target: { value: "X-FOO" } });
    fireEvent.change(valueInputs[2], { target: { value: "second" } });

    fireEvent.click(screen.getByText("创建"));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(onSaved).toHaveBeenLastCalledWith({
      name: "k",
      value: undefined,
      headers: { "X-FOO": "second" },
    });
  });

  it("点「填充 Codex 预设」注入种子行；再点不重复堆行（按名覆盖合并，Rulings 2）", () => {
    renderDialog(null);
    fireEvent.click(screen.getByText("填充 Codex 预设"));
    const afterFirst = (screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[]).map((i) => i.value);
    // 独立期望值：种子清单来自 new-api channel_affinity_setting.go 的 codex 组
    expect(afterFirst).toContain("Originator");
    expect(afterFirst).toContain("User-Agent");

    fireEvent.click(screen.getByText("填充 Codex 预设"));
    const afterSecond = (screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[]).map((i) => i.value);
    expect(afterSecond.length).toBe(afterFirst.length);
  });

  it("切换预设互斥（Rulings 2 修订）：Codex→Claude 不残留 Codex 独有头，手配非清单行保留", () => {
    renderDialog(null);
    fireEvent.click(screen.getByText("添加出站头"));
    fireEvent.change(screen.getAllByPlaceholderText("Header 名")[0], { target: { value: "X-Mine" } });
    fireEvent.change(screen.getAllByPlaceholderText("值")[0], { target: { value: "kept" } });

    fireEvent.click(screen.getByText("填充 Codex 预设"));
    expect((screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[]).map((i) => i.value)).toContain("Originator");

    fireEvent.click(screen.getByText("填充 Claude Code 预设"));
    const keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    const valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    const keys = keyInputs.map((i) => i.value);
    // Codex 独有头整行移除；Claude 独有头就位；两预设共有头只剩一行且值为 Claude 侧
    expect(keys).not.toContain("Originator");
    expect(keys).not.toContain("Session_id");
    expect(keys).toContain("X-App");
    const userAgentIndices = keys.filter((k) => k.toLowerCase() === "user-agent");
    expect(userAgentIndices).toHaveLength(1);
    expect(valueInputs[keys.indexOf("User-Agent")].value).toContain("claude-cli/");
    // 手配行不撞任何预设清单 → 互斥切换不误伤
    expect(keys).toContain("X-Mine");
  });

  it("预设与手配行撞名（大小写不敏感）：预设覆盖该行的名与值，其余手配行保留", () => {
    renderDialog(null);
    fireEvent.click(screen.getByText("添加出站头"));
    let keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    let valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    fireEvent.change(keyInputs[0], { target: { value: "user-agent" } });
    fireEvent.change(valueInputs[0], { target: { value: "my-agent" } });
    fireEvent.click(screen.getByText("添加出站头"));
    keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    fireEvent.change(keyInputs[1], { target: { value: "X-Mine" } });
    fireEvent.change(valueInputs[1], { target: { value: "kept" } });

    fireEvent.click(screen.getByText("填充 Claude Code 预设"));

    keyInputs = screen.getAllByPlaceholderText("Header 名") as HTMLInputElement[];
    valueInputs = screen.getAllByPlaceholderText("值") as HTMLInputElement[];
    const uaIndex = keyInputs.findIndex((i) => i.value.toLowerCase() === "user-agent");
    expect(uaIndex).toBeGreaterThanOrEqual(0);
    // 覆盖后只有一行 UA（合并非追加），且值换成预设值、名规范为预设拼写
    expect(keyInputs.filter((i) => i.value.toLowerCase() === "user-agent").length).toBe(1);
    expect(valueInputs[uaIndex].value).toContain("claude-cli/");
    expect(keyInputs.map((i) => i.value)).toContain("X-Mine");
  });

  it("「清空」一次移除全部行，保存得 {}（grilling 选 A：批量清空，不做按出身分组/撤销栈）", async () => {
    const { onSaved } = renderDialog(KEYED_HEADERS_ITEM);
    expect(screen.getAllByPlaceholderText("Header 名")).toHaveLength(2);

    fireEvent.click(screen.getByText("填充 Codex 预设"));
    expect(screen.getAllByPlaceholderText("Header 名").length).toBeGreaterThan(2);

    fireEvent.click(screen.getByText("清空"));
    expect(screen.queryAllByPlaceholderText("Header 名")).toHaveLength(0);
    // 空区时按钮不再渲染（无意义操作面不留）
    expect(screen.queryByText("清空")).toBeNull();

    fireEvent.click(screen.getByText("保存"));
    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ headers: {} })),
    );
  });

  it("删行按钮移除对应行", async () => {
    const { onSaved } = renderDialog(KEYED_HEADERS_ITEM);
    const removes = screen.getAllByLabelText("删除出站头");
    fireEvent.click(removes[0]);

    fireEvent.click(screen.getByText("保存"));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    const payload = onSaved.mock.calls.at(-1)?.[0] as { headers: Record<string, string> };
    expect(payload.headers).toEqual({ "X-App": "cli" });
  });
});
