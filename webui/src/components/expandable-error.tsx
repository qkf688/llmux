import { useState } from "react";
import { ChevronDown, ChevronRight, AlertCircle, CheckCircle, Info } from "lucide-react";
import { cn } from "@/lib/utils";

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

interface ExpandableErrorProps {
  /** 错误信息对象 */
  error: ErrorDetail;
  /** 是否默认展开 */
  defaultExpanded?: boolean;
  /** 自定义CSS类名 */
  className?: string;
  /** 显示成功状态（绿色）而不是错误状态（红色） */
  isSuccess?: boolean;
}

/** 获取错误类型的显示配置 */
function getErrorTypeConfig(type?: string) {
  const configs = {
    network: {
      icon: AlertCircle,
      color: "text-destructive",
      bgColor: "bg-destructive-tint",
      borderColor: "border-destructive/20",
      label: "网络错误",
    },
    auth: {
      icon: AlertCircle,
      color: "text-destructive",
      bgColor: "bg-destructive-tint",
      borderColor: "border-destructive/20",
      label: "认证错误",
    },
    provider: {
      icon: AlertCircle,
      color: "text-info",
      bgColor: "bg-info-tint",
      borderColor: "border-info/20",
      label: "提供商错误",
    },
    timeout: {
      icon: AlertCircle,
      color: "text-warning",
      bgColor: "bg-warning-tint",
      borderColor: "border-warning/20",
      label: "超时错误",
    },
    validation: {
      icon: AlertCircle,
      color: "text-info",
      bgColor: "bg-info-tint",
      borderColor: "border-info/20",
      label: "验证错误",
    },
    unknown: {
      icon: AlertCircle,
      color: "text-muted-foreground",
      bgColor: "bg-muted",
      borderColor: "border-border",
      label: "未知错误",
    },
  };

  return configs[type as keyof typeof configs] || configs.unknown;
}

/** 根据错误信息智能推断错误类型 */
function inferErrorType(message: string): ErrorDetail["type"] {
  const lowerMessage = message.toLowerCase();
  
  if (lowerMessage.includes("connection") || 
      lowerMessage.includes("network") || 
      lowerMessage.includes("dial") ||
      lowerMessage.includes("econnrefused") ||
      lowerMessage.includes("timeout")) {
    return "network";
  }
  
  if (lowerMessage.includes("unauthorized") || 
      lowerMessage.includes("401") || 
      lowerMessage.includes("api key") ||
      lowerMessage.includes("apikey") ||
      lowerMessage.includes("authentication") ||
      lowerMessage.includes("credential")) {
    return "auth";
  }
  
  if (lowerMessage.includes("not found") || 
      lowerMessage.includes("404") ||
      lowerMessage.includes("provider") ||
      lowerMessage.includes("base url")) {
    return "provider";
  }
  
  if (lowerMessage.includes("timeout") || 
      lowerMessage.includes("timed out")) {
    return "timeout";
  }
  
  if (lowerMessage.includes("invalid") || 
      lowerMessage.includes("validation") ||
      lowerMessage.includes("bad request") ||
      lowerMessage.includes("400")) {
    return "validation";
  }
  
  return "unknown";
}

// 成功状态展示块（ExpandableError / SimpleError 共用，避免重复）
function SuccessState({ message, className }: { message?: string; className?: string }) {
  return (
    <div className={cn(
      "rounded-md bg-success-tint border border-success/20 p-4",
      className
    )}>
      <div className="flex items-start gap-3">
        <CheckCircle className="h-5 w-5 text-success mt-0.5" />
        <div className="flex-1">
          <p className="text-sm text-success-foreground font-medium">
            {message || "操作成功"}
          </p>
        </div>
      </div>
    </div>
  );
}

export function ExpandableError({
  error,
  defaultExpanded = false,
  className,
  isSuccess = false
}: ExpandableErrorProps) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  // 如果是成功状态，显示成功样式
  if (isSuccess) {
    return <SuccessState message={error.summary} className={className} />;
  }

  // 推断错误类型
  const detectedType = error.type || inferErrorType(error.message);
  const typeConfig = getErrorTypeConfig(detectedType);
  const Icon = typeConfig.icon;
  
  // 构建概要文本
  const summaryText = error.summary || error.message.split('\n')[0];

  return (
    <div className={cn(
      "rounded-md border overflow-hidden transition-all duration-200",
      typeConfig.bgColor,
      typeConfig.borderColor,
      className
    )}>
      {/* 概要层 - 始终显示 */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full px-4 py-3 flex items-start gap-3 text-left hover:opacity-90 transition-opacity"
      >
        <Icon className={cn("h-5 w-5 mt-0.5 flex-shrink-0", typeConfig.color)} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className={cn("font-medium", typeConfig.color)}>
              {typeConfig.label}
            </span>
          </div>
          <p className="text-sm text-foreground mt-1 line-clamp-2">
            {summaryText}
          </p>
        </div>
        <div className="flex-shrink-0 flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            {isExpanded ? "收起" : "展开"}
          </span>
          {isExpanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
        </div>
      </button>

      {/* 详情层 - 可展开 */}
      {isExpanded && (
        <div className="border-t border-border/50 px-4 py-3 space-y-3">
          {/* 错误代码 */}
          {error.code && (
            <div className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">错误代码:</span>
              <code className="px-1.5 py-0.5 bg-background/50 rounded font-mono text-foreground">
                {error.code}
              </code>
            </div>
          )}

          {/* 完整错误信息 */}
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <Info className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="text-xs font-medium text-muted-foreground">详细信息</span>
            </div>
            <pre className="text-xs text-muted-foreground bg-background/50 rounded p-3 overflow-x-auto whitespace-pre-wrap break-all max-h-48">
              {error.message}
            </pre>
          </div>

        </div>
      )}
    </div>
  );
}

/** 简单错误展示组件 - 不带展开功能 */
export function SimpleError({ 
  error, 
  className,
  isSuccess = false 
}: ExpandableErrorProps) {
  if (isSuccess) {
    return <SuccessState message={error.summary} className={className} />;
  }

  const detectedType = error.type || inferErrorType(error.message);
  const typeConfig = getErrorTypeConfig(detectedType);
  const Icon = typeConfig.icon;

  return (
    <div className={cn(
      "rounded-md border p-4",
      typeConfig.bgColor,
      typeConfig.borderColor,
      className
    )}>
      <div className="flex items-start gap-3">
        <Icon className={cn("h-5 w-5 mt-0.5 flex-shrink-0", typeConfig.color)} />
        <div className="flex-1">
          <p className={cn("font-medium", typeConfig.color)}>
            {typeConfig.label}
          </p>
          <p className="text-sm text-foreground mt-1">
            {error.summary || error.message}
          </p>
        </div>
      </div>
    </div>
  );
}
