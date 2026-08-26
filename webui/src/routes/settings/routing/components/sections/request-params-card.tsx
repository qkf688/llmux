import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoutingSettingsSectionProps } from "../../types";

/**
 * 请求参数卡：全局超时/重试的四项设置（替代原 per-model time_out/max_retry）。
 * 语义分区明确——响应头超时只管"等头"，总超时是整请求预算帽子，
 * 首字节等待是流式"头后无数据"的看门狗窗口；等头超时不再一击致命
 * （总预算独立，后续候选仍有完整窗口，见 service/chat/chat_balance.go）。
 */
const intFields: Array<{
  key: "request_header_timeout" | "request_total_timeout" | "stream_first_byte_timeout" | "request_max_retry";
  label: string;
  description: string;
  suffix?: string;
}> = [
  {
    key: "request_header_timeout",
    label: "响应头超时",
    description: "单次尝试等待上游响应头的最长时间（秒），超时算该候选失败、可重试。",
    suffix: "秒",
  },
  {
    key: "request_total_timeout",
    label: "总超时",
    description: "整个请求的预算上限（秒），含所有重试与虚拟模型故障转移；到点整请求中止。",
    suffix: "秒",
  },
  {
    key: "stream_first_byte_timeout",
    label: "首字节等待",
    description: "流式响应头到达后，等待首个数据字节的最长时间（秒）；超时向客户端发送错误事件并中止。",
    suffix: "秒",
  },
  {
    key: "request_max_retry",
    label: "最大重试",
    description: "单个候选池的最大尝试次数，失败后按优先级/权重换下一个候选。",
    suffix: "次",
  },
];

export function RequestParamsCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>请求参数</CardTitle>
        <CardDescription>全局超时与重试配置（所有模型统一生效）</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        {intFields.map(({ key, label, description, suffix }) => (
          <div key={key} className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor={`request-params-${key}`} className="text-sm md:text-base font-medium">
                {label}
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">{description}</p>
            </div>
            <div className="flex items-center gap-2">
              <Input
                id={`request-params-${key}`}
                type="number"
                min={1}
                className="w-24 text-right"
                value={localSettings ? Number(localSettings[key]) : ""}
                onChange={(event) => {
                  const value = Number(event.target.value);
                  if (!Number.isNaN(value)) {
                    updateLocalSettings({ [key]: value });
                  }
                }}
              />
              {suffix && <span className="text-sm text-muted-foreground">{suffix}</span>}
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}