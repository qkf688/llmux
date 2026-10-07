import { describe, expect, it } from "vitest";
import { credentialToggleAction } from "./credential-status";

/**
 * 启停按钮的唯一判据（Stage 40）。
 *
 * 规则按「现在能不能用」而不是「是不是人工关的」：用不了的显示「启用」，
 * 能用的显示「停用」。冷却中的那条也显示「启用」——人点了就是要它立刻能用。
 */
describe("credentialToggleAction", () => {
  it("正常用着的显示「停用」、点下去切到停用", () => {
    expect(credentialToggleAction("active", null)).toEqual({ label: "停用", next: "disabled" });
  });

  it("空状态按可用处理（与后端选路把空串视同 active 一致）", () => {
    expect(credentialToggleAction("", null)).toEqual({ label: "停用", next: "disabled" });
  });

  it("冷却中的显示「启用」而不是「停用」", () => {
    const until = new Date(Date.now() + 60_000).toISOString();
    expect(credentialToggleAction("active", until)).toEqual({ label: "启用", next: "active" });
  });

  it("冷却已到期的不按冷却算（到期即自动可用）", () => {
    const until = new Date(Date.now() - 60_000).toISOString();
    expect(credentialToggleAction("active", until)).toEqual({ label: "停用", next: "disabled" });
  });

  it.each(["disabled", "temp_unsched", "error"])("不可用状态 %s 显示「启用」", (status) => {
    expect(credentialToggleAction(status, null)).toEqual({ label: "启用", next: "active" });
  });

  it("不可用状态的冷却时间不影响判断（已经用不了，显示仍是「启用」）", () => {
    const until = new Date(Date.now() + 60_000).toISOString();
    expect(credentialToggleAction("temp_unsched", until)).toEqual({ label: "启用", next: "active" });
  });
});