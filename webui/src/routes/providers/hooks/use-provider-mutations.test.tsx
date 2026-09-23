import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import type { ReactNode } from "react";
import type { UseFormReturn } from "react-hook-form";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Provider } from "@/lib/api";

const mocks = vi.hoisted(() => ({
  createProvider: vi.fn(),
  updateProvider: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  createProvider: mocks.createProvider,
  updateProvider: mocks.updateProvider,
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { useProviderMutations } from "./use-provider-mutations";
import { defaultProviderFormValues, type ProviderFormValues } from "../form-schema";

// 本文件锁前端写载荷的认证方式口径（Stage 20 消灭双写）：顶层 auth_type 键已随
// 后端写侧一并删除——认证方式只进 config（anthropic 用 auth_type、openai 系用
// openai_auth_type），顶层 AuthType 是后端派生的只读展示值，前端写它只会造漂移。

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
);

const formStub = { reset: vi.fn() } as unknown as UseFormReturn<ProviderFormValues>;

function renderMutations(editingProvider: Provider | null) {
  return renderHook(
    () =>
      useProviderMutations({
        form: formStub,
        editingProvider,
        setEditingProvider: vi.fn(),
        setOpen: vi.fn(),
      }),
    { wrapper },
  );
}

const anthropicValues: ProviderFormValues = {
  ...defaultProviderFormValues,
  name: "p-payload",
  type: "anthropic",
  protocols: ["anthropic"],
  auth_type: "x-api-key",
};

describe("provider 写载荷认证方式口径（Stage 20）", () => {
  beforeEach(() => {
    mocks.createProvider.mockReset().mockResolvedValue({});
    mocks.updateProvider.mockReset().mockResolvedValue({});
  });

  it("创建载荷不含顶层 auth_type（只进 config）", async () => {
    const { result } = renderMutations(null);
    await act(async () => {
      await result.current.handleSubmitProvider(anthropicValues);
    });

    expect(mocks.createProvider).toHaveBeenCalledTimes(1);
    const payload = mocks.createProvider.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("auth_type");
    const config = JSON.parse(payload.config as string) as Record<string, unknown>;
    expect(config.auth_type).toBe("x-api-key");
  });

  it("更新载荷不含顶层 auth_type（漂移面已随列删除）", async () => {
    const editing = { ID: 7 } as Provider;
    const { result } = renderMutations(editing);
    await act(async () => {
      await result.current.handleSubmitProvider(anthropicValues);
    });

    expect(mocks.updateProvider).toHaveBeenCalledTimes(1);
    const payload = mocks.updateProvider.mock.calls[0][1] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("auth_type");
  });
});
