import type { ReactNode } from "react";

type DetailCardProps = {
  label: string;
  value: ReactNode;
  mono?: boolean;
};

export function DetailCard({ label, value, mono = false }: DetailCardProps) {
  return (
    <div className="rounded-md border bg-muted/20 p-2 sm:p-3 space-y-1">
      <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
      <div className={`text-sm break-words ${mono ? "font-mono text-xs" : ""}`}>{value ?? "-"}</div>
    </div>
  );
}
