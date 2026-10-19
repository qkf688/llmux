import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// 一键写入的**调用方**契约：只能走窄端点。若有人改回 updateModelProvider（全量 PUT），
// 该关联的能力/thinking/请求头透传会被静默清掉（code-review Critical）——后端用例只锁
// 窄端点本身，锁不住调用方选错接口。
const apiMocks = vi.hoisted(() => ({
  createModelProvider: vi.fn(),
  updateModelProvider: vi.fn(),
  deleteModelProvider: vi.fn(),
  updateModelProviderProtocols: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { useModelProvidersAssociationMutations } from "./use-model-providers-association-mutations";

function renderMutations() {
  return renderHook(() =>
    useModelProvidersAssociationMutations({
      form: {} as never,
      buildPayload: vi.fn(),
      selectedModelId: 1,
      settings: null,
      fetchModelProviders: vi.fn(),
      isSubmitting: false,
      setIsSubmitting: vi.fn(),
      selectedProviderModels: [],
      setSelectedProviderModels: vi.fn(),
      setOpen: vi.fn(),
      editingAssociation: null,
      setEditingAssociation: vi.fn(),
      deleteId: null,
      setDeleteId: vi.fn(),
    }),
  );
}

describe("applyProtocolRecommendation", () => {
  it("整项覆盖推荐协议，且只调窄端点（不触碰其它字段）", async () => {
    const { result } = renderMutations();

    await act(async () => {
      await result.current.applyProtocolRecommendation(42, "anthropic");
    });

    expect(apiMocks.updateModelProviderProtocols).toHaveBeenCalledWith(42, {
      protocols: ["anthropic"],
      primary_protocol: "anthropic",
    });
    expect(apiMocks.updateModelProvider).not.toHaveBeenCalled();
  });
});