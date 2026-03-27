import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { LogsSettingsSectionProps } from "../../types";

export function PerformanceOptimizationCard({ localSettings, updateLocalSettings }: LogsSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>性能优化</CardTitle>
        <CardDescription>通过关闭部分功能来提升系统性能和降低资源消耗</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="disable-performance-tracking" className="text-sm md:text-base font-medium">
              关闭性能追踪
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              关闭后，系统将不再记录首包时间和 TPS（每秒 token 数）等性能指标。
              <br />
              可减少时间计算和统计开销，适度提升性能。
            </p>
          </div>
          <Switch
            id="disable-performance-tracking"
            checked={localSettings?.disable_performance_tracking ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ disable_performance_tracking: checked });
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="disable-token-counting" className="text-sm md:text-base font-medium">
              关闭 Token 统计
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              关闭后，系统将不再统计和记录 token 使用量（输入/输出 token 数）。
              <br />
              可减少 JSON 解析和字段提取开销，适度提升性能。
            </p>
          </div>
          <Switch
            id="disable-token-counting"
            checked={localSettings?.disable_token_counting ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ disable_token_counting: checked });
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="enable-request-trace" className="text-sm md:text-base font-medium">
              启用请求追踪
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统将使用 HTTP 追踪来监控网络请求的详细信息（如首字节时间）。
              <br />
              关闭可减少少量追踪开销，但会影响调试能力。建议保持开启。
            </p>
          </div>
          <Switch
            id="enable-request-trace"
            checked={localSettings?.enable_request_trace ?? true}
            onCheckedChange={(checked) => {
              updateLocalSettings({ enable_request_trace: checked });
            }}
          />
        </div>

        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="strip-response-headers" className="text-sm md:text-base font-medium">
              移除不必要的响应头
            </Label>
            <p className="text-xs md:text-sm text-muted-foreground">
              开启后，系统将只保留核心响应头（Content-Type、X-Request-Id 等），移除其他响应头。
              <br />
              可减少网络传输数据量，适度提升性能。
            </p>
          </div>
          <Switch
            id="strip-response-headers"
            checked={localSettings?.strip_response_headers ?? false}
            onCheckedChange={(checked) => {
              updateLocalSettings({ strip_response_headers: checked });
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

