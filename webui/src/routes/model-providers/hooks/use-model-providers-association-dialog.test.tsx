import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { createMockAssociation } from "../test-fixtures";
import { useModelProvidersAssociationDialog } from "./use-model-providers-association-dialog";
import { useModelProvidersAssociationForm } from "./use-model-providers-association-form";

// 组合真实的 form hook 与 dialog hook：回填逻辑的正确性取决于两者协作，
// mock form 会把 reset 的实际行为（含默认值合并）测掉。
function renderDialogHook() {
  return renderHook(() => {
    const { form } = useModelProvidersAssociationForm();
    const dialog = useModelProvidersAssociationDialog({
      form,
      settings: null,
      selectedModelId: 1,
      setOpen: vi.fn(),
      setEditingAssociation: vi.fn(),
      setSelectedProviderModels: vi.fn(),
    });
    return { form, ...dialog };
  });
}

describe("useModelProvidersAssociationDialog 的 thinking_levels 回填", () => {
  it("继承态（ThinkingLevels 为 null）回填为 inherit", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(createMockAssociation(1, 10, { ThinkingLevels: null }));
    });

    expect(result.current.form.getValues("thinking_levels_mode")).toBe("inherit");
    expect(result.current.form.getValues("thinking_levels_custom")).toEqual([]);
  });

  it("override 态（非空白名单）回填为 custom 并带上白名单", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(
        createMockAssociation(1, 10, { ThinkingLevels: ["low", "high"] }),
      );
    });

    expect(result.current.form.getValues("thinking_levels_mode")).toBe("custom");
    expect(result.current.form.getValues("thinking_levels_custom")).toEqual(["low", "high"]);
  });

  it("显式不约束（空数组）回填为 custom + 空白名单，不退化成 inherit", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(createMockAssociation(1, 10, { ThinkingLevels: [] }));
    });

    // 空数组与 null 语义不同：前者是"任意档位透传"的显式选择，必须能在表单里如实回显
    expect(result.current.form.getValues("thinking_levels_mode")).toBe("custom");
    expect(result.current.form.getValues("thinking_levels_custom")).toEqual([]);
  });

  it("键缺失（undefined）时兜底为 inherit，不误判 custom", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(createMockAssociation(1, 10, { ThinkingLevels: undefined }));
    });

    // 当前后端契约保证该键存在，此用例是**契约漂移的兜底回归**：
    // 若响应键名变更或被 omitempty 省略，误判 custom 会让保存把"继承"改写成"不约束"，
    // 静默破坏钳制规则。宽松判等把这类故障降级为"显示继承"。
    expect(result.current.form.getValues("thinking_levels_mode")).toBe("inherit");
  });
});
