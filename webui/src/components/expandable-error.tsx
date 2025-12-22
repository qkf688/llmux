import { useState } from "react";
import { ChevronDown, ChevronRight, AlertCircle, CheckCircle, Info, Lightbulb } from "lucide-react";
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
      color: "text-red-600",
      bgColor: "bg-red-50",
      borderColor: "border-red-200",
      label: "网络错误",
    },
    auth: {
      icon: AlertCircle,
      color: "text-orange-600",
      bgColor: "bg-orange-50",
      borderColor: "border-orange-200",
      label: "认证错误",
    },
    provider: {
      icon: AlertCircle,
      color: "text-blue-600",
      bgColor: "bg-blue-50",
      borderColor: "border-blue-200",
      label: "提供商错误",
    },
    timeout: {
      icon: AlertCircle,
      color: "text-yellow-600",
      bgColor: "bg-yellow-50",
      borderColor: "border-yellow-200",
      label: "超时错误",
    },
    validation: {
      icon: AlertCircle,
      color: "text-purple-600",
      bgColor: "bg-purple-50",
      borderColor: "border-purple-200",
      label: "验证错误",
    },
    unknown: {
      icon: AlertCircle,
      color: "text-gray-600",
      bgColor: "bg-gray-50",
      borderColor: "border-gray-200",
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

/** 生成通用解决建议 */
function generateSuggestions(type: ErrorDetail["type"], message: string): string[] {
  const lowerMessage = message.toLowerCase();
  const suggestions: string[] = [];

  switch (type) {
    case "network":
      suggestions.push("检查提供商 URL 是否正确");
      suggestions.push("确认网络连接正常");
      if (lowerMessage.includes("timeout")) {
        suggestions.push("尝试增加超时时间设置");
      }
      suggestions.push("检查防火墙或代理设置");
      break;
      
    case "auth":
      suggestions.push("验证 API Key 是否正确");
      suggestions.push("检查 API Key 是否有足够的权限");
      suggestions.push("确认 API Key 未过期");
      suggestions.push("检查提供商的控制台页面");
      break;
      
    case "provider":
      suggestions.push("检查提供商的配置信息");
      suggestions.push("确认 Base URL 格式正确");
      suggestions.push("验证提供商类型是否匹配");
      suggestions.push("查看提供商文档确认支持的模型");
      break;
      
    case "timeout":
      suggestions.push("网络连接较慢，耐心等待或重试");
      suggestions.push("检查网络连接稳定性");
      suggestions.push("尝试减少请求数据量");
      break;
      
    case "validation":
      suggestions.push("检查请求参数格式是否正确");
      suggestions.push("确认模型名称拼写无误");
      suggestions.push("查看 API 文档确认必需参数");
      break;
      
    default:
      suggestions.push("检查配置信息是否正确");
      suggestions.push("查看服务器日志获取更多详情");
      suggestions.push("尝试重新操作");
  }

  return suggestions;
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
    return (
      <div className={cn(
        "rounded-md bg-green-50 border border-green-200 p-4",
        className
      )}>
        <div className="flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-green-600 mt-0.5" />
          <div className="flex-1">
            <p className="text-sm text-green-800 font-medium">
              {error.summary || "操作成功"}
            </p>
          </div>
        </div>
      </div>
    );
  }

  // 推断错误类型
  const detectedType = error.type || inferErrorType(error.message);
  const typeConfig = getErrorTypeConfig(detectedType);
  const Icon = typeConfig.icon;
  const suggestions = error.suggestions || generateSuggestions(detectedType, error.message);
  
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
          <p className="text-sm text-gray-700 mt-1 line-clamp-2">
            {summaryText}
          </p>
        </div>
        <div className="flex-shrink-0 flex items-center gap-2">
          <span className="text-xs text-gray-500">
            {isExpanded ? "收起" : "展开"}
          </span>
          {isExpanded ? (
            <ChevronDown className="h-4 w-4 text-gray-500" />
          ) : (
            <ChevronRight className="h-4 w-4 text-gray-500" />
          )}
        </div>
      </button>

      {/* 详情层 - 可展开 */}
      {isExpanded && (
        <div className="border-t border-gray-200/50 px-4 py-3 space-y-3">
          {/* 错误代码 */}
          {error.code && (
            <div className="flex items-center gap-2 text-xs">
              <span className="text-gray-500">错误代码:</span>
              <code className="px-1.5 py-0.5 bg-white/50 rounded font-mono text-gray-700">
                {error.code}
              </code>
            </div>
          )}

          {/* 完整错误信息 */}
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <Info className="h-3.5 w-3.5 text-gray-500" />
              <span className="text-xs font-medium text-gray-600">详细信息</span>
            </div>
            <pre className="text-xs text-gray-600 bg-white/50 rounded p-3 overflow-x-auto whitespace-pre-wrap break-all max-h-48">
              {error.message}
            </pre>
          </div>

          {/* 解决建议 */}
          {suggestions.length > 0 && (
            <div>
              <div className="flex items-center gap-2 mb-1.5">
                <Lightbulb className="h-3.5 w-3.5 text-amber-500" />
                <span className="text-xs font-medium text-gray-600">解决建议</span>
              </div>
              <ul className="text-xs text-gray-700 space-y-1">
                {suggestions.map((suggestion, index) => (
                  <li key={index} className="flex items-start gap-2">
                    <span className="text-gray-400 mt-0.5">•</span>
                    <span>{suggestion}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
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
    return (
      <div className={cn(
        "rounded-md bg-green-50 border border-green-200 p-4",
        className
      )}>
        <div className="flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-green-600 mt-0.5" />
          <div className="flex-1">
            <p className="text-sm text-green-800 font-medium">
              {error.summary || "操作成功"}
            </p>
          </div>
        </div>
      </div>
    );
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
          <p className="text-sm text-gray-700 mt-1">
            {error.summary || error.message}
          </p>
        </div>
      </div>
    </div>
  );
}
