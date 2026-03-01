import type { ReactNode } from "react";

type MobileInfoItemProps = {
  label: string;
  value: ReactNode;
};

export const MobileInfoItem = ({ label, value }: MobileInfoItemProps) => (
  <div className="space-y-0.5">
    <p className="text-[11px] text-muted-foreground uppercase tracking-wide">{label}</p>
    <div className="text-xs font-medium break-words">{value}</div>
  </div>
);

