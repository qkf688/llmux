import { Button } from "@/components/ui/button";
import { getLogDiff, type ChatLog, type DiffResult } from "@/lib/api";
import { toErrorMessage } from "@/lib/errors";
import { ChevronDown, ChevronRight, GitCompare } from "lucide-react";
import { useState } from "react";

type TransformDiffSectionProps = {
  log: ChatLog;
};

function formatValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "(空)";
  }
  if (typeof value === "string") {
    return value.length > 100 ? value.slice(0, 100) + "..." : value;
  }
  const str = JSON.stringify(value);
  return str.length > 100 ? str.slice(0, 100) + "..." : str;
}

function DiffGroup({
  title,
  entries,
  colorClass,
  labelClass,
}: {
  title: string;
  entries: { path: string; raw: unknown; after: unknown }[];
  colorClass: string;
  labelClass: string;
}) {
  if (entries.length === 0) {
    return null;
  }
  return (
    <div className={`rounded-md border p-2 space-y-1 sm:p-3 ${colorClass}`}>
      <p className={`text-[11px] uppercase tracking-wide ${labelClass}`}>
        {title} ({entries.length})
      </p>
      <div className="space-y-1 max-h-40 overflow-y-auto">
        {entries.map((e) => (
          <div key={e.path} className="text-xs font-mono break-all">
            <span className={labelClass}>{e.path}</span>
            {e.raw !== null && e.raw !== undefined && (
              <span className="text-muted-foreground"> : {formatValue(e.raw)}</span>
            )}
            {e.after !== null && e.after !== undefined && (
              <span className="text-muted-foreground"> → {formatValue(e.after)}</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function TransformDiffSection({ log }: TransformDiffSectionProps) {
  const [expanded, setExpanded] = useState(false);
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 只在两份请求体都存在时才展示
  if (!log.RawRequestBody || !log.RequestBody) {
    return null;
  }

  const handleToggle = () => {
    if (!expanded && !diff && !loading) {
      setLoading(true);
      setError(null);
      void (async () => {
        try {
          const result = await getLogDiff(log.ID);
          setDiff(result);
        } catch (err) {
          setError(toErrorMessage(err));
        } finally {
          setLoading(false);
        }
      })();
    }
    setExpanded(!expanded);
  };

  const diffCount = diff
    ? diff.lost_fields.length + diff.added_fields.length + diff.changed_values.length
    : 0;
  const hasDiff = diffCount > 0;

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={handleToggle} disabled={loading}>
          {expanded ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
          <GitCompare className="size-4 mr-1" />
          {loading ? "分析中..." : "转换差异"}
        </Button>
        {diff && hasDiff && (
          <span className="text-xs text-muted-foreground">
            {diffCount} 处差异
          </span>
        )}
        {diff && !hasDiff && (
          <span className="text-xs text-green-600">无差异</span>
        )}
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 p-2 sm:p-3">
          <p className="text-xs text-destructive uppercase tracking-wide">加载差异失败</p>
          <div className="text-destructive whitespace-pre-wrap break-words text-sm">{error}</div>
        </div>
      )}

      {expanded && diff && hasDiff && (
        <div className="space-y-2">
          <DiffGroup
            title="丢失字段（raw 有、转换后没有）"
            entries={diff.lost_fields}
            colorClass="bg-red-50 dark:bg-red-950/20"
            labelClass="text-red-700 dark:text-red-400"
          />
          <DiffGroup
            title="新增字段（转换后新增）"
            entries={diff.added_fields}
            colorClass="bg-green-50 dark:bg-green-950/20"
            labelClass="text-green-700 dark:text-green-400"
          />
          <DiffGroup
            title="值变化（同名字段值不同）"
            entries={diff.changed_values}
            colorClass="bg-amber-50 dark:bg-amber-950/20"
            labelClass="text-amber-700 dark:text-amber-400"
          />
        </div>
      )}
    </div>
  );
}
