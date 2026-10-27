import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { RoutingSettingsSectionProps } from "../../types";

/**
 * 凭据冷却卡：凭据级失败的冷却窗口、鉴权失败判停阈值与惰性探活间隔（S4 状态机 #6-1/#6-2/#6-3）。
 * 429 限流独立窗口；5xx/超时/网络共用服务端窗口；鉴权失败(401/403)不走冷却，
 * 连续失败达阈值后判停 temp_unsched（选路剔除）；探活间隔控制 temp_unsched 自愈频控。
 * 每项配一个开关（**默认关**：自动冷却/自动停用不默认生效，凭据不会被系统擅自停用，
 * 需要时在设置页手动开启）。关掉后该机制完全不生效——凭据不再被自动冷却或自动停用，
 * 下方的窗口/阈值数值此时不生效（保留原值，开关重新打开即按原值生效）。
 * 改动逐请求读库即时生效。
 */
type CredHealthValueKey =
  | "cred_health_cooldown_429_sec"
  | "cred_health_cooldown_server_sec"
  | "cred_health_auth_fail_threshold"
  | "cred_health_probe_interval_sec";

type CredHealthToggleKey =
  | "cred_health_cooldown_429_enabled"
  | "cred_health_cooldown_server_enabled"
  | "cred_health_auth_fail_stop_enabled"
  | "cred_health_probe_enabled";

const rows: Array<{
  valueKey: CredHealthValueKey;
  toggleKey: CredHealthToggleKey;
  toggleId: string;
  label: string;
  description: string;
  suffix: string;
}> = [
  {
    valueKey: "cred_health_cooldown_429_sec",
    toggleKey: "cred_health_cooldown_429_enabled",
    toggleId: "cred-cooldown-429-enabled",
    label: "429 限流冷却",
    description:
      "开启后，触发 429 限流的凭据按下方时长冷却（窗口内选路剔除、到期自动恢复）；关闭后 429 不再改动凭据状态。",
    suffix: "秒",
  },
  {
    valueKey: "cred_health_cooldown_server_sec",
    toggleKey: "cred_health_cooldown_server_enabled",
    toggleId: "cred-cooldown-server-enabled",
    label: "服务端错误冷却",
    description:
      "开启后，5xx / 超时 / 网络错误会让凭据按下方时长冷却（窗口内选路剔除、到期自动恢复）；关闭后不再冷却。",
    suffix: "秒",
  },
  {
    valueKey: "cred_health_auth_fail_threshold",
    toggleKey: "cred_health_auth_fail_stop_enabled",
    toggleId: "cred-cooldown-auth-fail-enabled",
    label: "鉴权失败判停",
    description:
      "开启后，连续鉴权失败（401/403）达下方次数即判停凭据并剔除选路，需恢复（探活/手动）后才再次选用；若「惰性探活」也关闭，被判停的凭据不会自动恢复，只能人工改回启用。关闭后只记录失败次数，不会自动停用凭据（期间次数照记，重新开启后按已记次数判定，可能立即判停）。",
    suffix: "次",
  },
  {
    valueKey: "cred_health_probe_interval_sec",
    toggleKey: "cred_health_probe_enabled",
    toggleId: "cred-cooldown-probe-enabled",
    label: "惰性探活",
    description:
      "开启后，判停凭据（temp_unsched）按下方间隔探活上游并自动恢复；关闭后判停凭据不再自动恢复，只能人工改回启用。",
    suffix: "秒",
  },
];

export function CredentialCooldownCard({ localSettings, updateLocalSettings }: RoutingSettingsSectionProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>凭据冷却</CardTitle>
        <CardDescription>
          凭据级失败处理：冷却窗口、鉴权失败判停阈值与惰性探活间隔（所有供应商统一生效）。开关关掉后对应机制完全不生效。
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 md:space-y-6">
        {rows.map(({ valueKey, toggleKey, toggleId, label, description, suffix }) => {
          const enabled = localSettings ? localSettings[toggleKey] : false;
          return (
            <div key={valueKey} className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor={toggleId} className="text-sm md:text-base font-medium">
                  {label}
                </Label>
                <p className="text-xs md:text-sm text-muted-foreground">{description}</p>
              </div>
              <div className="flex items-center gap-2">
                <Input
                  id={`cred-cooldown-${valueKey}`}
                  aria-label={`${label}数值`}
                  type="number"
                  min={1}
                  disabled={!enabled}
                  className="w-24 text-right"
                  value={localSettings ? Number(localSettings[valueKey]) : ""}
                  onChange={(event) => {
                    const value = Number(event.target.value);
                    if (!Number.isNaN(value)) {
                      updateLocalSettings({ [valueKey]: value });
                    }
                  }}
                />
                <span className="text-sm text-muted-foreground">{suffix}</span>
                <Switch
                  id={toggleId}
                  checked={enabled}
                  onCheckedChange={(checked) => {
                    updateLocalSettings({ [toggleKey]: checked });
                  }}
                />
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
