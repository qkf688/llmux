import type { GroupRef } from "@/lib/api";

/** 分组徽标平铺（C2 显示层）：每组一个、max-width truncate、无组时不渲染。 */
export function GroupBadges({ groups }: { groups: GroupRef[] | undefined }) {
  if (!groups || groups.length === 0) return null;
  return (
    <div className="flex items-center gap-1 flex-wrap min-w-0">
      {groups.map((g) => (
        <span
          key={g.ID}
          className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-muted text-muted-foreground max-w-[120px] truncate"
          title={g.Name}
        >
          {g.Name}
        </span>
      ))}
    </div>
  );
}
