import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Model } from "@/lib/api";
import { createMockAssociation } from "../test-fixtures";
import { useModelProvidersAssociationDialog } from "./use-model-providers-association-dialog";
import { useModelProvidersAssociationForm } from "./use-model-providers-association-form";

// 模型能力真值夹具：三项刻意取互不相同（true/false/true），
// 这样"继承态勾选初值 = 模型真值"写错成别的值时，三个断言能各自指出是哪一项错了。
const MODELS_WITH_CAPS: Model[] = [
  {
    ID: 1,
    Name: "cap-model",
    Remark: "",
    IOLog: false,
    auto_associate: true,
    supports_thinking: false,
    tool_call: true,
    structured_output: false,
    image: true,
  },
];

// 组合真实的 form hook 与 dialog hook：回填逻辑的正确性取决于两者协作，
// mock form 会把 reset 的实际行为（含默认值合并）测掉。
function renderDialogHook(models: Model[] = MODELS_WITH_CAPS) {
  return renderHook(() => {
    const { form } = useModelProvidersAssociationForm();
    const dialog = useModelProvidersAssociationDialog({
      form,
      settings: null,
      selectedModelId: 1,
      models,
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

describe("useModelProvidersAssociationDialog 的能力回填", () => {
  it("三项全 nil → 总控落在 inherit", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(
        createMockAssociation(1, 10, { ToolCall: null, StructuredOutput: null, Image: null }),
      );
    });

    expect(result.current.form.getValues("capabilities_mode")).toBe("inherit");
  });

  it("任一项有 override → 总控落在 custom（混合态不退化成 inherit）", () => {
    const { result } = renderDialogHook();

    act(() => {
      // 只有 image 显式禁用，tool/structured 仍继承——二值开关若只判"全 nil"以外一律 custom，
      // 混合态才能原样回显；反过来若把 custom 当 inherit，这项 override 会被静默抹掉。
      result.current.openEditDialog(
        createMockAssociation(1, 10, { ToolCall: null, StructuredOutput: null, Image: false }),
      );
    });

    expect(result.current.form.getValues("capabilities_mode")).toBe("custom");
    // 继承项的勾选初值取模型真值，override 项取其自身值
    expect(result.current.form.getValues("tool_call")).toBe(true);
    expect(result.current.form.getValues("structured_output")).toBe(false);
    expect(result.current.form.getValues("image")).toBe(false);
  });

  it("全部 override → 总控落在 custom 且勾选如实回显", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(
        createMockAssociation(1, 10, { ToolCall: true, StructuredOutput: false, Image: true }),
      );
    });

    expect(result.current.form.getValues("capabilities_mode")).toBe("custom");
    expect(result.current.form.getValues("tool_call")).toBe(true);
    expect(result.current.form.getValues("structured_output")).toBe(false);
    expect(result.current.form.getValues("image")).toBe(true);
  });

  // 这条锁死"静默翻转"：继承态下把初值写成 false，用户只切一下总控就保存，
  // 三项能力会从"跟随模型（可能全开）"变成"全禁用"——界面上看不出任何异常。
  it("继承态的勾选初值取模型真值，切到自定义不改变生效能力", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openEditDialog(
        createMockAssociation(1, 10, { ToolCall: null, StructuredOutput: null, Image: null }),
      );
    });

    // 夹具模型 tool_call=true / structured_output=false / image=true
    expect(result.current.form.getValues("tool_call")).toBe(true);
    expect(result.current.form.getValues("structured_output")).toBe(false);
    expect(result.current.form.getValues("image")).toBe(true);
  });

  it("新建关联默认 inherit，勾选初值同样取模型真值", () => {
    const { result } = renderDialogHook();

    act(() => {
      result.current.openCreateDialog();
    });

    expect(result.current.form.getValues("capabilities_mode")).toBe("inherit");
    expect(result.current.form.getValues("tool_call")).toBe(true);
    expect(result.current.form.getValues("structured_output")).toBe(false);
    expect(result.current.form.getValues("image")).toBe(true);
  });
});
