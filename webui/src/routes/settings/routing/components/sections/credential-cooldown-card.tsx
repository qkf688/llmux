import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoutingSettingsSectionProps } from "../../types";

/**
 * 凭据冷却卡：凭据级失败的冷却窗口、鉴权失败判停阈值与惰性探活间隔（S4 状态机 #6-1/#6-2/#6-3）。
 * 429 限流独立窗口；5xx/超时/网络共用服务端窗口；鉴权失败(401/403)不走冷却，
 * 连续失败达阈值后判停 temp_unsched（选路剔除）；探活间隔控制 temp_unsched 自愈频控。
 * 改动逐请求读库即时生效。
 */
const intFields: Array<{
  key:
    | "cred_health_cooldown_429_sec"
    | "cred_health_cooldown_server_sec"
    | "cred_health_auth_fail_threshold"
    | "cred_health_probe_interval_sec";
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
    description: "5xx / 超时 / 网络错误后凭据冷却的时长（秒），窗口内选路剔除、到期自动恢复。",
    suffix: "秒",
  },
  {
    key: "cred_health_auth_fail_threshold",
    label: "鉴权失败判停阈值",
    description: "连续鉴权失败（401/403）达此次数后凭据判停并选路剔除，需恢复（探活/手动）后才会再次选用。",
    suffix: "次",
  },
  {
    key: "cred_health_probe_interval_sec",
    label: "惰性探活间隔",
    description: "判停凭据（temp_unsched）两次探活之间的最小间隔（秒）；间隔内不重探上游。",
    suffix: "秒",
  },
];

export function CredentialCooldownCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>凭据冷却</CardTitle>
        <CardDescription>凭据级失败处理：冷却窗口、鉴权失败判停阈值与惰性探活间隔（所有供应商统一生效）</CardDescription>
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