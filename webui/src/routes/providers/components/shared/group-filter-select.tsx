import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { GroupRef } from "@/lib/api";
import { GROUP_FILTER_SOLO } from "@/lib/provider-models";

interface GroupFilterSelectProps {
  groups: GroupRef[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

/** 分组筛选下拉（C2 显示层）：全部分组 / 各组 / 仅单分组可服务。 */
export function GroupFilterSelect({
  groups,
  value,
  onChange,
  disabled,
}: GroupFilterSelectProps) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger className="w-36" size="sm" aria-label="按分组筛选">
        <SelectValue placeholder="全部分组" />
      </SelectTrigger>
      <SelectContent align="start" className="min-w-[9rem]">
        <SelectItem value="all">全部分组</SelectItem>
        {groups.map((g) => (
          <SelectItem key={g.ID} value={String(g.ID)}>
            {g.Name}
          </SelectItem>
        ))}
        {groups.length > 0 && (
          <SelectItem value={GROUP_FILTER_SOLO}>仅单分组可服务</SelectItem>
        )}
      </SelectContent>
    </Select>
  );
}
