import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ModelSyncDetailDialog } from "./model-sync-detail-dialog";
import { copyTextWithToast } from "@/lib/clipboard";
import type { ModelSyncLog } from "@/lib/api";

// 复制结果的三级降级提示由 lib/clipboard 承载（内部经 sonner 上屏），这里只断
// 「点了哪个 chip、交给它哪段文本、成功文案有没有带上模型名标识」这三件事。
// 必须走 lib/clipboard 而不是自己读剪贴板：三级提示是它的单一出处，绕过它
// 就等于把这个契约复制一份到页面里。
vi.mock("@/lib/clipboard", () => ({
  copyTextWithToast: vi.fn().mockResolvedValue(true),
}));

const makeLog = (p: Partial<ModelSyncLog>): ModelSyncLog => ({
  ID: 1,
  ProviderID: 7,
  ProviderName: "fu",
  Status: "success",
  Error: "",
  AddedCount: 2,
  RemovedCount: 1,
  AddedModels: ["big-pickle", "nvidia/nemotron-3-ultra-550b-a55b:free"],
  RemovedModels: ["laguna-m.1"],
  SyncedAt: "2026-10-01T03:25:45Z",
  ...p,
});

function renderDialog(log: ModelSyncLog) {
  render(<ModelSyncDetailDialog log={log} onClose={() => {}} />);
}

describe("ModelSyncDetailDialog 模型名点击复制", () => {
  beforeEach(() => {
    vi.mocked(copyTextWithToast).mockClear();
  });

  it("点新增模型 chip 复制该模型名，成功文案带上模型名标识", () => {
    renderDialog(makeLog({}));
    fireEvent.click(screen.getByRole("button", { name: "big-pickle" }));
    expect(copyTextWithToast).toHaveBeenCalledTimes(1);
    expect(copyTextWithToast).toHaveBeenCalledWith("big-pickle", "已复制模型名: big-pickle");
  });

  it("删除模型 chip 同样可复制（两处共用 ModelChipList，不各自复制一份逻辑）", () => {
    renderDialog(makeLog({}));
    fireEvent.click(screen.getByRole("button", { name: "laguna-m.1" }));
    expect(copyTextWithToast).toHaveBeenCalledWith("laguna-m.1", "已复制模型名: laguna-m.1");
  });

  // 命名空间与 :free 后缀是识别模型的关键信息，chip 一旦截断就再也认不出是哪个模型。
  it("长模型名整串作为可点击热区，不截断", () => {
    renderDialog(makeLog({}));
    const chip = screen.getByRole("button", { name: "nvidia/nemotron-3-ultra-550b-a55b:free" });
    fireEvent.click(chip);
    expect(copyTextWithToast).toHaveBeenCalledWith(
      "nvidia/nemotron-3-ultra-550b-a55b:free",
      "已复制模型名: nvidia/nemotron-3-ultra-550b-a55b:free",
    );
  });
});