import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useForm, FormProvider, useFormContext, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

// GroupsSection/GroupCard 未渲染，但 use-pool-options 在模块顶层被 import；
// mock usePools 避免拉起 QueryClient 依赖。poolState 让用例能摆不同的号池列表
// （手建池 / 专属池），并模拟「列表已就绪」与「列表请求失败」两种状态。
const { poolState } = vi.hoisted(() => ({
  poolState: {
    data: [] as Array<{ ID: number; Name: string; KeyCount: number; AutoCreated: boolean }>,
    isFetched: true,
    isError: false,
  },
}));
vi.mock("@/hooks/api", () => ({
  usePools: () => poolState,
}));

import {
  SupportTypesField,
  EndpointsSection,
  GroupsSection,
} from "./provider-schedule-fields";
import { linkablePoolOptions, resolveGroupMode } from "../../utils/group-mode";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  defaultProviderFormValues,
  providerFormSchema,
  type ProviderFormValues,
} from "../../form-schema";

/** 探针：把 watch 到的 protocols / endpoints 协议序列渲染成可读文本，供断言表单真实值 */
function Probe() {
  const { control } = useFormContext<ProviderFormValues>();
  const protocols = useWatch({ control, name: "protocols" }) ?? [];
  const endpoints = useWatch({ control, name: "endpoints" }) ?? [];
  return (
    <div>
      <span data-testid="probe-protocols">{JSON.stringify(protocols)}</span>
      <span data-testid="probe-endpoints">
        {JSON.stringify(endpoints.map((e) => e.protocol))}
      </span>
    </div>
  );
}

function Harness() {
  // 复刻 openCreateDialog：默认预勾选 OpenAI + type 同步为 openai
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: { ...defaultProviderFormValues, type: "openai" },
  });
  return (
    // 组件内 Radix Tooltip 依赖 Provider context（App 根已挂载，测试 Harness 需自备，
    // 与 api-key-form-dialog.test.tsx 姿势一致）
    <TooltipProvider>
      <FormProvider {...form}>
        <SupportTypesField providerTemplates={[]} />
        <EndpointsSection />
        <Probe />
      </FormProvider>
    </TooltipProvider>
  );
}

/** 端点行数 = 「继承 Base URL」Badge 数量（每行默认态恰好一个，文案唯一不与描述段冲突） */
function endpointRowCount(): number {
  return screen.queryAllByText("继承 Base URL", { exact: true }).length;
}

describe("支持类型勾选 → 协议端点同步（添加供应商弹窗）", () => {
  it("初始态：预勾选 OpenAI，端点区恰好 1 行", () => {
    render(<Harness />);
    expect(screen.getByTestId("probe-protocols").textContent).toBe('["openai"]');
    expect(endpointRowCount()).toBe(1);
  });

  it("勾选第 2 个类型（OpenAI Responses）：protocols 与端点行都 +1", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));

    expect(screen.getByTestId("probe-protocols").textContent).toContain("responses");
    expect(screen.getByTestId("probe-endpoints").textContent).toContain("responses");
    expect(endpointRowCount()).toBe(2);
  });

  it("勾选全部 3 个类型：端点区应显示 3 行（复现「不同步增加」）", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Anthropic/ }));

    // 表单真实值：3 个协议 + 3 条端点
    expect(screen.getByTestId("probe-protocols").textContent).toBe(
      '["openai","responses","anthropic"]',
    );
    expect(screen.getByTestId("probe-endpoints").textContent).toBe(
      '["openai","responses","anthropic"]',
    );
    // 显示层：端点区必须渲染 3 行——用户报告的正是这里没跟上
    expect(endpointRowCount()).toBe(3);
  });

  it("取消勾选协议：对应端点行同步移除（setValue filter 路径）", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));
    expect(endpointRowCount()).toBe(2);

    // 再次点击同一 checkbox = 取消勾选
    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));

    expect(screen.getByTestId("probe-endpoints").textContent).toBe('["openai"]');
    expect(endpointRowCount()).toBe(1);
  });

  it("取消勾选再重新勾选：端点行回到「继承 Base URL」态（covering 局部状态随行卸载重置）", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));
    expect(endpointRowCount()).toBe(2);

    // 把 responses 行（最后一条）切到覆盖态：继承 Badge 只剩 openai 一个
    const coverButtons = screen.getAllByText("覆盖 URL");
    fireEvent.click(coverButtons[coverButtons.length - 1]);
    expect(endpointRowCount()).toBe(1);

    // 取消勾选 → 行卸载、covering 随之消失；重新勾选 → 全新行应回到继承态
    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /OpenAI Responses/ }));
    expect(endpointRowCount()).toBe(2);
  });
});

/** 分组区 Harness：探针额外 watch groups[].poolId / mode，供断言回填后的表单真实值 */
function GroupsHarness({
  onAddKeys,
  groups,
}: {
  onAddKeys?: (index: number) => void;
  groups?: ProviderFormValues["groups"];
}) {
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: {
      ...defaultProviderFormValues,
      type: "openai",
      groups: groups ?? [
        { name: "组一", weight: 1, poolId: "" },
        { name: "组二", weight: 2, poolId: "" },
      ],
    },
  });
  const { control } = form;
  const pools = useWatch({ control, name: "groups" }) ?? [];
  return (
    <TooltipProvider>
      <FormProvider {...form}>
        <GroupsSection onAddKeys={onAddKeys} />
        <span data-testid="probe-pool-ids">{JSON.stringify(pools.map((g) => g?.poolId ?? ""))}</span>
        <span data-testid="probe-modes">{JSON.stringify(pools.map((g) => g?.mode ?? ""))}</span>
      </FormProvider>
    </TooltipProvider>
  );
}

/** 复位号池列表（用例间互不污染） */
function setPools(pools: Array<{ ID: number; Name: string; KeyCount: number; AutoCreated: boolean }>) {
  poolState.data = pools;
  poolState.isFetched = true;
  poolState.isError = false;
}

describe("凭据分组区 - 取 Key 模式（直接添加 / 关联号池）", () => {
  it("默认「直接添加」：选中该模式、显示「添加 Key」按钮、不出现号池下拉", () => {
    setPools([]);
    render(<GroupsHarness />);

    const direct = screen.getAllByRole("radio", { name: "直接添加" });
    expect(direct).toHaveLength(2);
    expect(direct[0].getAttribute("aria-checked")).toBe("true");
    expect(screen.getAllByRole("radio", { name: "关联号池" })[0].getAttribute("aria-checked")).toBe(
      "false",
    );
    expect(screen.getAllByRole("button", { name: /添加 Key/ })).toHaveLength(2);
    // 直接添加模式下不出现号池下拉（凭据由系统自动建池承载）
    expect(screen.queryAllByText("选择号池（凭据在号池页管理）")).toHaveLength(0);
  });

  it("点「添加 Key」带上该组的下标回调（回填要按组定位，不能默认落到第一组）", () => {
    setPools([]);
    const onAddKeys = vi.fn();
    render(<GroupsHarness onAddKeys={onAddKeys} />);

    const buttons = screen.getAllByRole("button", { name: /添加 Key/ });
    expect(buttons).toHaveLength(2);

    fireEvent.click(buttons[1]);

    expect(onAddKeys).toHaveBeenCalledTimes(1);
    expect(onAddKeys).toHaveBeenCalledWith(1);
  });

  it("未传 onAddKeys 时按钮仍渲染但点击不抛错（回调是可选注入）", () => {
    setPools([]);
    render(<GroupsHarness />);
    fireEvent.click(screen.getAllByRole("button", { name: /添加 Key/ })[0]);
    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["",""]');
  });

  it("切到「关联号池」：出现号池下拉、隐藏「添加 Key」按钮（Key 不归本表单管）", () => {
    setPools([{ ID: 7, Name: "手建池", KeyCount: 3, AutoCreated: false }]);
    render(<GroupsHarness />);

    fireEvent.click(screen.getAllByRole("radio", { name: "关联号池" })[0]);

    expect(screen.getAllByRole("radio", { name: "关联号池" })[0].getAttribute("aria-checked")).toBe(
      "true",
    );
    // 只有被切换的第一组进入关联模式（第二组仍是直接添加）
    expect(screen.getAllByText("选择号池（凭据在号池页管理）")).toHaveLength(1);
    expect(screen.queryAllByRole("button", { name: /添加 Key/ })).toHaveLength(1);
  });

  it("绑的是手建池时默认落在「关联号池」；切到「直接添加」会解绑（专属池只收自己的 Key）", () => {
    setPools([{ ID: 7, Name: "手建池", KeyCount: 3, AutoCreated: false }]);
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "7" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    // 手建池 → 关联号池模式
    expect(screen.getAllByRole("radio", { name: "关联号池" })[0].getAttribute("aria-checked")).toBe(
      "true",
    );

    fireEvent.click(screen.getAllByRole("radio", { name: "直接添加" })[0]);

    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["",""]');
    expect(screen.getByTestId("probe-modes").textContent).toBe('["direct",""]');
  });

  it("绑的是自动池（专属池）时默认落在「直接添加」，且保留绑定不解绑", () => {
    setPools([{ ID: 9, Name: "供应商A·默认组", KeyCount: 2, AutoCreated: true }]);
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "9" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    expect(screen.getAllByRole("radio", { name: "直接添加" })[0].getAttribute("aria-checked")).toBe(
      "true",
    );
    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["9",""]');
  });

  it("号池列表请求失败：不猜模式（不渲染 radio），并提示刷新重试", () => {
    setPools([{ ID: 9, Name: "供应商A·默认组", KeyCount: 2, AutoCreated: true }]);
    poolState.isError = true;
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "9" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    // 列表没拿到 ≠ 池被删：这时翻成「关联号池」是用错误模式解释真实数据。
    // 组一绑着池（模式取决于列表）→ 不渲染 radio、只给占位提示；
    // 组二没绑池（模式恒为「直接添加」，与列表无关）→ radio 照常渲染。
    expect(screen.getAllByRole("radio")).toHaveLength(2);
    expect(screen.getAllByText("号池列表加载失败，请刷新后重试")).toHaveLength(1);
    // 绑定值不受影响（提交仍按真实 pool_id 走）
    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["9",""]');
  });

  it("池还没进列表（刚建好等刷新）：切到「直接添加」不解绑（解绑=丢掉刚建的专属池）", () => {
    // 列表里没有该池：模拟「刚建好、列表还没刷回来」的窗口
    setPools([]);
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "9" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    fireEvent.click(screen.getAllByRole("radio", { name: "直接添加" })[0]);

    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["9",""]');
    expect(screen.getByTestId("probe-modes").textContent).toBe('["direct",""]');
  });

  it("link 模式下绑的是专属池：下拉显示占位、不列专属池（专属池不是可关联项）", () => {
    setPools([{ ID: 9, Name: "供应商A·默认组", KeyCount: 2, AutoCreated: true }]);
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "9" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    fireEvent.click(screen.getAllByRole("radio", { name: "关联号池" })[0]);

    // 专属池不出现在关联下拉里：走占位文案，页面上也不再出现该池名
    expect(screen.getAllByText("选择号池（凭据在号池页管理）").length).toBeGreaterThan(0);
    expect(screen.queryByText(/供应商A·默认组/)).toBeNull();
    // 并说清专属池为什么不在列表里（否则用户以为被漏了）
    expect(screen.getAllByText(/专属号池不会出现在关联列表里/).length).toBeGreaterThan(0);
    // 绑定不被清掉：切回「直接添加」还能接着用自己的专属池
    expect(screen.getByTestId("probe-pool-ids").textContent).toBe('["9",""]');
    // 「查看详情」禁用：当前没关联可关联的号池（专属池不在关联列表里），
    // 点了会打开自己专属池的详情，看着像「查看的是我关联的那个池」，实际不是。
    const detailButton = screen.getAllByRole("button", { name: /查看详情/ })[0] as HTMLButtonElement;
    expect(detailButton.disabled).toBe(true);
  });

  it("link 模式下关联了手建池：「查看详情」可点（看的是真正关联的那个池）", () => {
    setPools([{ ID: 7, Name: "手建池", KeyCount: 3, AutoCreated: false }]);
    render(
      <GroupsHarness
        groups={[
          { name: "组一", weight: 1, poolId: "7" },
          { name: "组二", weight: 2, poolId: "" },
        ]}
      />,
    );

    // 组一绑手建池 → 推导即「关联号池」，详情按钮可点
    const detailButton = screen.getAllByRole("button", { name: /查看详情/ })[0] as HTMLButtonElement;
    expect(detailButton.disabled).toBe(false);
  });
});

describe("link 模式可选号池（专属池不进关联选项）", () => {
  const manual = { id: 7, name: "手建池", keyCount: 3, autoCreated: false };
  const auto = { id: 9, name: "供应商A·默认组", keyCount: 2, autoCreated: true };

  it("只列手建池：专属池不作为共享池被关联（否则两组共用一批凭据）", () => {
    expect(linkablePoolOptions([manual, auto])).toEqual([manual]);
  });

  it("当前绑定的专属池也不进选项：专属池一律不作为可关联项", () => {
    // 旧版会把当前绑定的专属池留在选项里，代价是没有手建池时下拉里只剩它自己，
    // 看着像「专属池可以关联」——实际是死路，用户会以为专属池被共享了。
    expect(linkablePoolOptions([manual, auto])).not.toContain(auto);
    expect(linkablePoolOptions([auto])).toEqual([]);
  });
});

describe("分组取 Key 模式推导（纯逻辑）", () => {
  it("列表没就绪/失败时返回未知（null），不猜模式", () => {
    expect(resolveGroupMode("9", undefined, false)).toBeNull();
  });

  it("绑的是专属池 → 直接添加；绑的是手建池 → 关联号池", () => {
    expect(resolveGroupMode("9", { id: 9, name: "p", keyCount: 1, autoCreated: true }, true)).toBe(
      "direct",
    );
    expect(resolveGroupMode("7", { id: 7, name: "p", keyCount: 1, autoCreated: false }, true)).toBe(
      "link",
    );
  });
});
