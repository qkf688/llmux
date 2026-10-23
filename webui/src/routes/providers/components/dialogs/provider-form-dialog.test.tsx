import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { useForm, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

// 供应商弹窗的「添加 Key」编排（建池 + 回填）没有别的测试覆盖：
// 后端端点有强用例，但「提交后回填 groups.N.poolId 与 mode」这一步改坏了整套前端用例仍全绿。
// 这里 mock 掉数据层，只断编排本身：走哪个接口、回填了什么。
const { apiMock, poolState } = vi.hoisted(() => ({
  apiMock: {
    batchImport: vi.fn(),
    autoImport: vi.fn(),
  },
  poolState: {
    data: [] as Array<{ ID: number; Name: string; KeyCount: number; AutoCreated: boolean }>,
    isFetched: true,
    isError: false,
  },
}));

vi.mock("@/hooks/api", () => ({
  usePools: () => poolState,
  useBatchImportCredentials: () => ({ mutateAsync: apiMock.batchImport, isPending: false }),
  useAutoImportPoolCredentials: () => ({ mutateAsync: apiMock.autoImport, isPending: false }),
  // PoolDetailDialog 也会用到这批 hook（弹窗本身与本用例无关，给最小替身即可）
  usePoolCredentials: () => ({ data: undefined, isPending: false }),
  useUpdateCredential: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteCredential: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBatchUpdateCredentialStatus: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBatchDeleteCredentials: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

import { ProviderFormDialog } from "./provider-form-dialog";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  defaultProviderFormValues,
  providerFormSchema,
  type ProviderFormValues,
} from "../../form-schema";

function DialogHarness({
  formRef,
  groups,
}: {
  formRef: { current: UseFormReturn<ProviderFormValues> | null };
  groups?: ProviderFormValues["groups"];
}) {
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: {
      ...defaultProviderFormValues,
      name: "供应商A",
      type: "openai",
      groups: groups ?? [{ name: "组一", weight: 1, poolId: "" }],
    },
  });
  formRef.current = form;
  return (
    // Radix Tooltip 需要 Provider（App 根已挂载，测试 Harness 自备）
    <TooltipProvider>
      <ProviderFormDialog
        open
        onOpenChange={() => {}}
        editingProvider={null}
        detailLoading={false}
        form={form}
        providerTemplates={[]}
        watchedType="openai"
        watchedProtocols={["openai"]}
        onSubmit={() => {}}
      />
    </TooltipProvider>
  );
}

/** 走一遍「点添加 Key → 粘贴 → 点添加」 */
async function pasteKeys(text: string) {
  fireEvent.click(screen.getAllByRole("button", { name: /添加 Key/ })[0]);
  const textarea = document.querySelector("textarea");
  if (!textarea) throw new Error("找不到粘贴输入框");
  fireEvent.change(textarea, { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "添加" }));
}

describe("供应商弹窗「添加 Key」导入编排", () => {
  it("分组还没绑池：走「建池 + 导入」，并把新池 id 与模式回填到该分组", async () => {
    poolState.data = [];
    apiMock.autoImport.mockResolvedValue({
      pool_id: 7,
      pool_name: "供应商A·组一",
      auto_created: true,
      key_count: 2,
      total: 2,
      imported: 2,
      skipped: 0,
      failed: 0,
      rows: [],
    });
    const formRef = { current: null as UseFormReturn<ProviderFormValues> | null };
    render(<DialogHarness formRef={formRef} />);

    await pasteKeys("sk-1\nsk-2");

    await waitFor(() => expect(apiMock.autoImport).toHaveBeenCalledTimes(1));
    expect(apiMock.batchImport).not.toHaveBeenCalled();
    // 粘贴文本按行拆成 keys 数组交给后端（后端不做文本解析）
    expect(apiMock.autoImport.mock.calls[0][0].keys).toEqual(["sk-1", "sk-2"]);
    // 只传供应商名与分组名两个原语：池名与备注由服务端按统一口径生成，前端不参与命名
    expect(apiMock.autoImport.mock.calls[0][0].provider_name).toBe("供应商A");
    expect(apiMock.autoImport.mock.calls[0][0].group_name).toBe("组一");
    expect(apiMock.autoImport.mock.calls[0][0]).not.toHaveProperty("name");
    expect(apiMock.autoImport.mock.calls[0][0]).not.toHaveProperty("note");
    await waitFor(() => {
      expect(formRef.current?.getValues("groups.0.poolId")).toBe("7");
      expect(formRef.current?.getValues("groups.0.mode")).toBe("direct");
    });
  });

  it("分组已绑专属池：走「往该池追加」，不建池也不改绑定", async () => {
    poolState.data = [{ ID: 7, Name: "供应商A·组一", KeyCount: 1, AutoCreated: true }];
    apiMock.batchImport.mockResolvedValue({
      total: 1,
      imported: 1,
      skipped: 0,
      failed: 0,
      rows: [],
    });
    const formRef = { current: null as UseFormReturn<ProviderFormValues> | null };
    render(
      <DialogHarness
        formRef={formRef}
        groups={[{ name: "组一", weight: 1, poolId: "7" }]}
      />,
    );

    await pasteKeys("sk-3");

    await waitFor(() => expect(apiMock.batchImport).toHaveBeenCalledTimes(1));
    expect(apiMock.batchImport.mock.calls[0][0]).toEqual({ poolId: 7, keys: ["sk-3"] });
    expect(apiMock.autoImport).not.toHaveBeenCalled();
    expect(formRef.current?.getValues("groups.0.poolId")).toBe("7");
  });
});
