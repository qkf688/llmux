import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoutingSettingsSectionProps } from "../../types";

/**
 * 凭据冷却卡：凭据级失败的冷却窗口（#6-1 可配基数 + 按失败类型分窗）。
 * 429 限流独立窗口；5xx/超时/网络共用服务端窗口（鉴权失败过渡期同窗，
 * #6-2 判停落地后 401/403 不再走冷却）。改动逐请求读库即时生效。
 */
const intFields: Array<{
  key: "cred_health_cooldown_429_sec" | "cred_health_cooldown_server_sec";
  label: string;
  description: string;
  suffix?: string;
}> = [
  {
    key: "cred_health_cooldown_429_sec",
    label: "429 限流冷却",
    description: "触发 429 限流后凭据冷却的时长（秒），窗口内选路剔除、到期自动恢复。",
    suffix: "秒",
  },
  {
    key: "cred_health_cooldown_server_sec",
    label: "服务端错误冷却",
    description: "5xx / 超时 / 网络错误后凭据冷却的时长（秒）；鉴权失败过渡期同窗。",
    suffix: "秒",
  },
];

export function CredentialCooldownCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>凭据冷却</CardTitle>
        <CardDescription>凭据级失败后的冷却窗口（所有供应商统一生效）</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        {intFields.map(({ key, label, description, suffix }) => (
          <div key={key} className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor={`cred-cooldown-${key}`} className="text-sm md:text-base font-medium">
                {label}
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">{description}</p>
            </div>
            <div className="flex items-center gap-2">
              <Input
                id={`cred-cooldown-${key}`}
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