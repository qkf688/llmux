import { describe, expect, it } from "vitest";
import {
  credentialToggleAction,
  describeCredentialFailureReason,
  formatCredentialLastUsed,
  formatRelativeTime,
} from "./credential-status";

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

/**
 * 失败原因的机器码 → 中文映射。
 *
 * 这些机器码来自后端 service/chat 的 cooldownReasonForStatus / cooldownReasonAuthFail，
 * 是「凭据为什么不可用」在界面上唯一的解释来源。用例覆盖三条分支：已登记固定码、
 * http_<状态码> 兜底、空/未登记的原样露出（宁可难看也不藏线索）。
 */
describe("describeCredentialFailureReason", () => {
  it("已登记的固定码翻译成人话", () => {
    expect(describeCredentialFailureReason("auth_fail")).toContain("鉴权失败");
    expect(describeCredentialFailureReason("network")).toContain("网络");
    expect(describeCredentialFailureReason("http_429")).toContain("限流");
  });

  it("http_<状态码> 用前缀兜底，保留状态码本身", () => {
    expect(describeCredentialFailureReason("http_500")).toContain("500");
    expect(describeCredentialFailureReason("http_503")).toContain("503");
  });

  it("空值返回空串（调用方据此不渲染，不编造「未知原因」）", () => {
    expect(describeCredentialFailureReason("")).toBe("");
    expect(describeCredentialFailureReason(null)).toBe("");
    expect(describeCredentialFailureReason(undefined)).toBe("");
    expect(describeCredentialFailureReason("   ")).toBe("");
  });

  it("未登记的机器码原样露出，不吞成通用文案", () => {
    expect(describeCredentialFailureReason("some_new_code")).toBe("some_new_code");
  });
});

/**
 * 相对时间格式化内核。
 *
 * 两个时间口径（上次成功使用 / 上次探测）共用同一份时间算式，只有「没有值」的
 * 措辞不同——措辞由调用方给（neverLabel），故本内核不自带任何语义词。
 */
describe("formatRelativeTime", () => {
  it("没有值时用调用方给的措辞（探测语境是「从未探测」，不是「从未使用」）", () => {
    expect(formatRelativeTime(null, "从未探测")).toBe("从未探测");
    expect(formatRelativeTime(undefined, "从未探测")).toBe("从未探测");
    expect(formatRelativeTime("", "从未探测")).toBe("从未探测");
    // 非法串同样按「没有值」处理，不显示成 1970 年之类的假时间
    expect(formatRelativeTime("not-a-date", "从未探测")).toBe("从未探测");
  });

  it("近期按秒/分/小时/天分级，超过一周给日期", () => {
    const now = Date.now();
    expect(formatRelativeTime(new Date(now).toISOString(), "从未探测")).toBe("刚刚");
    expect(formatRelativeTime(new Date(now - 5 * 60_000).toISOString(), "从未探测")).toBe("5分钟前");
    expect(formatRelativeTime(new Date(now - 3 * 3_600_000).toISOString(), "从未探测")).toBe("3小时前");
    expect(formatRelativeTime(new Date(now - 2 * 86_400_000).toISOString(), "从未探测")).toBe("2天前");
    expect(formatRelativeTime(new Date(now - 30 * 86_400_000).toISOString(), "从未探测")).not.toMatch(/前$/);
  });

  it("上次成功使用的既有口径不变（仍是「从未使用」）", () => {
    expect(formatCredentialLastUsed(null)).toBe("从未使用");
    expect(formatCredentialLastUsed("not-a-date")).toBe("从未使用");
  });
});
