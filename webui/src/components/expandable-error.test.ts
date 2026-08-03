import { describe, expect, it } from "vitest";
import { AlertTriangle, Clock, HelpCircle, Lock, Server, WifiOff } from "lucide-react";

import { getErrorTypeConfig, inferErrorType } from "./expandable-error-utils";

describe("getErrorTypeConfig", () => {
  // 语义色三件套是稳定设计契约（destructive/info/warning 语义 token），精确断言不会因"抄错编号"失守
  it.each([
    { type: "network", color: "text-destructive", bgColor: "bg-destructive-tint", borderColor: "border-destructive/20" },
    { type: "provider", color: "text-info", bgColor: "bg-info-tint", borderColor: "border-info/20" },
    { type: "timeout", color: "text-warning", bgColor: "bg-warning-tint", borderColor: "border-warning/20" },
  ] as const)("$type 使用语义色三件套", ({ type, color, bgColor, borderColor }) => {
    const config = getErrorTypeConfig(type);
    expect(config.color).toBe(color);
    expect(config.bgColor).toBe(bgColor);
    expect(config.borderColor).toBe(borderColor);
  });

  // chart 色断言由 chartVar 模板驱动，锁定"引用正确的 chart 变量"而非逐字复制三件套字符串
  it.each(["chart-8", "chart-5"] as const)("auth/validation 使用独立 chart 色 %s", (chartVar) => {
    const config = getErrorTypeConfig(chartVar === "chart-8" ? "auth" : "validation");
    expect(config.color).toBe(`text-[color:var(--${chartVar})]`);
    expect(config.bgColor).toBe(`bg-[color:var(--${chartVar})]/15`);
    expect(config.borderColor).toBe(`border-[color:var(--${chartVar})]/20`);
  });

  it("unknown 使用中性回退（精确断言）", () => {
    const config = getErrorTypeConfig("unknown");
    expect(config.color).toBe("text-muted-foreground");
    expect(config.bgColor).toBe("bg-muted");
    expect(config.borderColor).toBe("border-border");
  });

  it("未知字符串回退到 unknown 配置", () => {
    const config = getErrorTypeConfig("not-a-real-type");
    expect(config.label).toBe("未知错误");
    expect(config.color).toBe("text-muted-foreground");
    expect(config.icon).toBe(HelpCircle);
  });

  it("undefined 回退到 unknown 配置", () => {
    const config = getErrorTypeConfig(undefined);
    expect(config.label).toBe("未知错误");
    expect(config.color).toBe("text-muted-foreground");
    expect(config.icon).toBe(HelpCircle);
  });

  it("六种错误类型颜色互不相同（保留语义区分度）", () => {
    const colors = new Set(
      ["network", "auth", "provider", "timeout", "validation", "unknown"].map(
        (type) => getErrorTypeConfig(type).color
      )
    );
    expect(colors.size).toBe(6);
  });

  // 图标精确断言锁定「类型→图标」绑定：图标是与颜色正交的第二区分通道（可访问性增强），
  // 引用 lucide 图标常量而非复制名称字符串，防止"抄错图标名"类假阳性
  it.each([
    { type: "network", icon: WifiOff },
    { type: "auth", icon: Lock },
    { type: "provider", icon: Server },
    { type: "timeout", icon: Clock },
    { type: "validation", icon: AlertTriangle },
    { type: "unknown", icon: HelpCircle },
  ] as const)("$type 使用独立语义图标", ({ type, icon }) => {
    expect(getErrorTypeConfig(type).icon).toBe(icon);
  });

  it("六种错误类型图标互不相同（形状通道兜底色觉缺失）", () => {
    const icons = new Set(
      ["network", "auth", "provider", "timeout", "validation", "unknown"].map(
        (type) => getErrorTypeConfig(type).icon
      )
    );
    expect(icons.size).toBe(6);
  });
});

describe("inferErrorType", () => {
  // 关键词优先级（有意锁定）：network > auth > provider > timeout > validation > unknown。
  // 混合关键词命中前序分支是当前设计的固化行为，改动优先级前必须先改这里的期望。
  const expectedInference = [
    { message: "connection refused", type: "network" },
    { message: "network unreachable", type: "network" },
    { message: "dial tcp 1.2.3.4:443", type: "network" },
    { message: "ECONNREFUSED", type: "network" },
    { message: "request timed out", type: "timeout" },
    { message: "timeout after 30s", type: "timeout" },
    { message: "unauthorized", type: "auth" },
    { message: "401 unauthorized", type: "auth" },
    { message: "invalid api key", type: "auth" },
    { message: "authentication failed", type: "auth" },
    { message: "invalid credential", type: "auth" },
    { message: "provider returned 500", type: "provider" },
    { message: "model not found", type: "provider" },
    { message: "404 not found", type: "provider" },
    { message: "invalid base url", type: "provider" },
    { message: "invalid request body", type: "validation" },
    { message: "validation error", type: "validation" },
    { message: "bad request", type: "validation" },
    { message: "400 bad request", type: "validation" },
    { message: "something unexpected happened", type: "unknown" },
  ] as const;

  it.each(expectedInference)("“$message” 推断为 $type", ({ message, type }) => {
    expect(inferErrorType(message)).toBe(type);
  });

  it("timeout 关键词不再被 network 分支拦截", () => {
    expect(inferErrorType("timeout")).toBe("timeout");
  });

  it("混合关键词命中前序分支（network 优先于 timeout）", () => {
    expect(inferErrorType("connection timeout")).toBe("network");
    expect(inferErrorType("dial tcp i/o timeout")).toBe("network");
  });

  it("混合关键词命中前序分支（auth/provider 优先于 timeout）", () => {
    expect(inferErrorType("unauthorized timeout")).toBe("auth");
    expect(inferErrorType("404 timeout")).toBe("provider");
  });

  it("混合关键词 timeout 优先于 validation", () => {
    expect(inferErrorType("invalid timeout")).toBe("timeout");
  });

  it("大小写不敏感", () => {
    expect(inferErrorType("TIMEOUT")).toBe("timeout");
    expect(inferErrorType("Unauthorized")).toBe("auth");
    expect(inferErrorType("CONNECTION REFUSED")).toBe("network");
  });

  it("中文错误消息回退到 unknown", () => {
    expect(inferErrorType("连接超时")).toBe("unknown");
  });
});
