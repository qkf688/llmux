import { formatCredentialLastUsed } from "@/components/credentials/credential-status";

type CredentialStatsProps = {
  totalRequests: number;
  totalErrors: number;
  lastUsedAt: string | null;
  className?: string;
};

/**
 * 凭据终身统计单行读数：`请求 N · 失败 M · 相对时间`（gpt-load 式紧凑格式）。
 * N=成功尝试数（200 被接受即记），M=凭据级失败数（终身累计，独立于连败判停计数）；
 * 时间为上次成功时刻，从未成功过显「从未使用」。桌面表格「统计」列与移动卡片共用。
 */
export function CredentialStats({
  totalRequests,
  totalErrors,
  lastUsedAt,
  className,
}: CredentialStatsProps) {
  return (
    <span
      data-testid="credential-stats"
      className={`whitespace-nowrap text-xs text-muted-foreground ${className ?? ""}`}
    >
      请求 <strong className="font-medium text-foreground">{totalRequests}</strong>
      {" · "}失败{" "}
      <strong className={`font-medium ${totalErrors > 0 ? "text-destructive" : "text-foreground"}`}>
        {totalErrors}
      </strong>
      {" · "}
      {formatCredentialLastUsed(lastUsedAt)}
    </span>
  );
}
