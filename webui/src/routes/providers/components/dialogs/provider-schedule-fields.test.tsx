import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useForm, FormProvider, useFormContext, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

// GroupsSection/GroupCard 未渲染，但 use-pool-options 在模块顶层被 import；
// mock usePools 避免拉起 QueryClient 依赖
vi.mock("@/hooks/api", () => ({
  usePools: () => ({ data: [] }),
}));

import { SupportTypesField, EndpointsSection } from "./provider-schedule-fields";
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
