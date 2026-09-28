/**
 * 凭据状态徽标：号池详情行与供应商编辑弹窗的内联 key 行共用同一渲染
 * （状态词汇与配色单源见 ./credential-status）。
 */
import {
  CREDENTIAL_STATUS_BADGE_CLS,
  CREDENTIAL_STATUS_LABEL,
  COOLDOWN_BADGE_CLS,
  isCredentialInCooldown,
} from "./credential-status";
import type { CredentialStatus } from "@/lib/api";

type CredentialStatusBadgeProps = {
  status: string;
  /** 冷却截止时间（ISO）；在冷却窗口内额外出一枚「冷却」徽标 */
  cooldownUntil?: string | null;
};

/** 已知状态走色板，未知状态原样展示（后端新增状态时不至于显示空白）。 */
export function CredentialStatusBadge({ status, cooldownUntil }: CredentialStatusBadgeProps) {
  const known = status in CREDENTIAL_STATUS_LABEL ? (status as CredentialStatus) : null;
  return (
    <div className="flex flex-wrap items-center gap-1">
      {known ? (
        <span
          className={`inline-flex rounded-full px-2 py-0.5 text-[10.5px] font-medium ${CREDENTIAL_STATUS_BADGE_CLS[known]}`}
        >
          {CREDENTIAL_STATUS_LABEL[known]}
        </span>
      ) : (
        <span className="inline-flex rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
          {status}
        </span>
      )}
      {isCredentialInCooldown(cooldownUntil) && (
        <span className={`inline-flex rounded-full px-2 py-0.5 text-[10.5px] font-medium ${COOLDOWN_BADGE_CLS}`}>
          冷却
        </span>
      )}
    </div>
  );
}