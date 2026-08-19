import { Badge } from "@/components/ui/badge";
import type { ChatLog } from "@/lib/api";

type UnclaimedFieldsSectionProps = {
  log: ChatLog;
};

/**
 * 展示入站原始请求体里本网关未认领的顶层键。
 *
 * 目前只渲染「已完成检测且确有未认领键」一种情况。其余三态（原始体未记录、
 * 该协议不支持检测、body 解析失败）一律不渲染，而**不是**画成「无问题」——
 * 把「查不了」显示成「已检查、无未认领键」是假阴性，比没有这个功能更糟。
 * 四态各自的文案与语义边界说明是后续独立一步的事，此处宁可不显示。
 *
 * 只列键名不带值预览：转换前 body 已在同一弹窗全文展示，且少一层敏感数据暴露面。
 */
export function UnclaimedFieldsSection({ log }: UnclaimedFieldsSectionProps) {
  const unclaimed = log.unclaimed_request_fields;

  if (!unclaimed || unclaimed.status !== "ok" || unclaimed.fields.length === 0) {
    return null;
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">未认领请求字段</p>
      <div className="rounded-md border bg-warning-tint p-2 space-y-2 sm:p-3">
        <p className="text-[11px] text-warning-foreground uppercase tracking-wide">
          本网关未解析的顶层键 ({unclaimed.fields.length})
        </p>
        <div className="flex flex-wrap gap-1.5">
          {unclaimed.fields.map((field) => (
            <Badge key={field} variant="outline" className="font-mono font-normal text-warning-foreground">
              {field}
            </Badge>
          ))}
        </div>
      </div>
    </div>
  );
}
