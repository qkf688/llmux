import {
  AlertTriangle,
  Clock,
  HelpCircle,
  Lock,
  Server,
  WifiOff,
} from "lucide-react";

export interface ErrorDetail {
  /** 错误类型：network | auth | provider | timeout | validation | unknown */
  type?: "network" | "auth" | "provider" | "timeout" | "validation" | "unknown";
  /** 简要的错误摘要（显示在概要层） */
  summary?: string;
  /** 完整的错误信息（显示在详情层） */
  message: string;
  /** 错误代码（如果有） */
  code?: string;
  /** 解决建议列表 */
  suggestions?: string[];
  /** 原始错误对象 */
  originalError?: Error;
}

/** 获取错误类型的显示配置 */
export function getErrorTypeConfig(type?: string) {
  // 颜色机制有意不一致：语义色用 -tint（color-mix 12%），chart-N 系列用 /15 透明度，
  // 与 capability-badges 先例保持一致，两者权重差异（~3%）是可接受的视觉取舍
  const configs = {
    network: {
      icon: WifiOff,
      color: "text-destructive",
      bgColor: "bg-destructive-tint",
      borderColor: "border-destructive/20",
      label: "网络错误",
    },
    auth: {
      icon: Lock,
      color: "text-[color:var(--chart-8)]",
      bgColor: "bg-[color:var(--chart-8)]/15",
      borderColor: "border-[color:var(--chart-8)]/20",
      label: "认证错误",
    },
    provider: {
      icon: Server,
      color: "text-info",
      bgColor: "bg-info-tint",
      borderColor: "border-info/20",
      label: "提供商错误",
    },
    timeout: {
      icon: Clock,
      color: "text-warning",
      bgColor: "bg-warning-tint",
      borderColor: "border-warning/20",
      label: "超时错误",
    },
    validation: {
      icon: AlertTriangle,
      color: "text-[color:var(--chart-5)]",
      bgColor: "bg-[color:var(--chart-5)]/15",
      borderColor: "border-[color:var(--chart-5)]/20",
      label: "验证错误",
    },
    unknown: {
      icon: HelpCircle,
      color: "text-muted-foreground",
      bgColor: "bg-muted",
      borderColor: "border-border",
      label: "未知错误",
    },
  };

  return configs[type as keyof typeof configs] || configs.unknown;
}

/** 根据错误信息智能推断错误类型 */
export function inferErrorType(message: string): ErrorDetail["type"] {
  const lowerMessage = message.toLowerCase();

  if (
    lowerMessage.includes("connection") ||
    lowerMessage.includes("network") ||
    lowerMessage.includes("dial") ||
    lowerMessage.includes("econnrefused")
  ) {
    return "network";
  }

  if (
    lowerMessage.includes("unauthorized") ||
    lowerMessage.includes("401") ||
    lowerMessage.includes("api key") ||
    lowerMessage.includes("apikey") ||
    lowerMessage.includes("authentication") ||
    lowerMessage.includes("credential")
  ) {
    return "auth";
  }

  if (
    lowerMessage.includes("not found") ||
    lowerMessage.includes("404") ||
    lowerMessage.includes("provider") ||
    lowerMessage.includes("base url")
  ) {
    return "provider";
  }

  if (lowerMessage.includes("timeout") || lowerMessage.includes("timed out")) {
    return "timeout";
  }

  if (
    lowerMessage.includes("invalid") ||
    lowerMessage.includes("validation") ||
    lowerMessage.includes("bad request") ||
    lowerMessage.includes("400")
  ) {
    return "validation";
  }

  return "unknown";
}
