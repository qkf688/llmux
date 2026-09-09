import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { CredentialTestOutcome } from "./use-credential-testing";

interface CredentialTestButtonProps {
  outcome: CredentialTestOutcome | undefined;
  keyMasked: string;
  onTest: () => void;
}

/**
 * 号池详情「按凭据测试」按钮（#24）：与 providers 域 ModelTestStatusButton 同视觉——
 * 未测灰圈 / loading 转圈 / 成功绿勾 / 失败红叉，tooltip 带结论。
 * 独立于此域而非跨域 import providers 组件：两者结果类型不同（此处是
 * CredentialTestOutcome，且失败 tooltip 额外带被测 key 掩码，帮管理员定位是哪把 key）。
 */
export function CredentialTestButton({ outcome, keyMasked, onTest }: CredentialTestButtonProps) {
  // 回显实际命中的链路（#24-B）：多引用组时让管理员看得见测的是哪个分组、用的哪个模型。
  const link = [
    outcome?.group ? `组 ${outcome.group}` : "",
    outcome?.model ? `模型 ${outcome.model}` : "",
  ]
    .filter(Boolean)
    .join(" · ");
  const linkSuffix = link ? ` · ${link}` : "";
  const tooltip = outcome?.loading
    ? "测试中..."
    : outcome?.success === true
      ? `凭据可用（${keyMasked}）${linkSuffix}`
      : outcome?.success === false
        ? `${keyMasked}: ${outcome.error || "测试失败"}${linkSuffix}`
        : "测试该凭据可用性";

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={onTest}
          disabled={outcome?.loading === true}
          aria-label={`测试凭据 ${keyMasked}`}
        >
          {outcome?.loading ? (
            <Spinner className="h-3.5 w-3.5" />
          ) : outcome?.success === true ? (
            <svg
              className="h-3.5 w-3.5 text-success"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          ) : outcome?.success === false ? (
            <svg
              className="h-3.5 w-3.5 text-destructive"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          ) : (
            <svg
              className="h-3.5 w-3.5 text-muted-foreground"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  );
}
