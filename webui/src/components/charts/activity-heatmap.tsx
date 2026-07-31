"use client";

import { Fragment, memo, useCallback, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { Activity, Bot, MessageSquare } from "lucide-react";
import type { DailyMetricsData } from "@/lib/api";
import { formatCompactCount } from "@/lib/formatters";

type ActivityDay = {
  dateStr: string; // YYYY-MM-DD
  isFuture: boolean;
  stat: DailyMetricsData | null;
};

const LEVELS = [
  { min: 5000, level: 4 },
  { min: 2000, level: 3 },
  { min: 1000, level: 2 },
  { min: 1, level: 1 },
] as const;

const LEGEND_LEVELS = [0, ...LEVELS.map((l) => l.level)] as const;

function getActivityLevel(value: number): number {
  if (value <= 0) return 0;
  return LEVELS.find((l) => value >= l.min)?.level ?? 1;
}

// 格子与图例共用同一色阶：比例由 LEVELS 档数推导，加档位时无需两处同步
function getLevelColor(level: number): string {
  if (level <= 0) return "var(--muted)";
  const stepPercent = 100 / LEVELS.length;
  return `color-mix(in oklch, var(--primary) ${level * stepPercent}%, var(--muted))`;
}

type TooltipState = {
  day: ActivityDay;
  x: number;
  y: number;
  source: "hover" | "focus";
};

type HeatmapCellProps = {
  day: ActivityDay;
  onShowTooltip: (day: ActivityDay, element: HTMLElement, source: TooltipState["source"]) => void;
  onHideTooltip: (day: ActivityDay, source: TooltipState["source"], relatedTarget: EventTarget | null) => void;
};

const HeatmapCell = memo(function HeatmapCell({ day, onShowTooltip, onHideTooltip }: HeatmapCellProps) {
  const rawValue = day.stat?.reqs ?? 0;
  const level = getActivityLevel(rawValue);
  const ariaLabel = day.stat
    ? `${day.dateStr}，请求 ${day.stat.reqs.toLocaleString()}，Token ${day.stat.tokens.toLocaleString()}`
    : day.dateStr;

  return (
    <button
      type="button"
      aria-label={ariaLabel}
      className="cursor-pointer rounded-sm border-0 bg-transparent p-0 ring-1 ring-border/20 transition-[transform,box-shadow] duration-150 origin-top hover:scale-150 hover:ring-primary/30 focus-visible:scale-150 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/50"
      onMouseEnter={(e) => onShowTooltip(day, e.currentTarget, "hover")}
      onMouseLeave={(e) => onHideTooltip(day, "hover", e.relatedTarget)}
      onFocus={(e) => onShowTooltip(day, e.currentTarget, "focus")}
      onBlur={(e) => onHideTooltip(day, "focus", e.relatedTarget)}
      style={{ backgroundColor: getLevelColor(level) }}
    />
  );
});

function formatDateYYYYMMDD(date: Date): string {
  const y = date.getFullYear();
  const m = `${date.getMonth() + 1}`.padStart(2, "0");
  const d = `${date.getDate()}`.padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function addDays(date: Date, days: number): Date {
  const copy = new Date(date);
  copy.setDate(copy.getDate() + days);
  return copy;
}

function startOfDay(date: Date): Date {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

export function ActivityHeatmapCard({
  data,
}: {
  data: DailyMetricsData[];
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [maskImage, setMaskImage] = useState("none");
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const [cellPx, setCellPx] = useState(14);

  const dayMap = useMemo(() => {
    const map = new Map<string, DailyMetricsData>();
    for (const item of data ?? []) {
      map.set(item.date, item);
    }
    return map;
  }, [data]);

  const days = useMemo<ActivityDay[]>(() => {
    const today = startOfDay(new Date());
    const jsDay = today.getDay(); // 0 = Sunday
    const startDate = addDays(today, -(jsDay + 53 * 7));

    const result: ActivityDay[] = [];
    for (let i = 0; i < 54 * 7; i++) {
      const current = addDays(startDate, i);
      const dateStr = formatDateYYYYMMDD(current);
      result.push({
        dateStr,
        isFuture: current.getTime() > today.getTime(),
        stat: dayMap.get(dateStr) ?? null,
      });
    }
    return result;
  }, [dayMap]);

  const checkScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const { scrollLeft, scrollWidth, clientWidth } = scrollRef.current;
    const isStart = scrollLeft <= 1;
    const isEnd = Math.abs(scrollWidth - clientWidth - scrollLeft) <= 1;

    if (isStart && isEnd) {
      setMaskImage("none");
    } else if (isStart) {
      setMaskImage("linear-gradient(to left, transparent, rgba(0,0,0,0) 10px, black 40px)");
    } else if (isEnd) {
      setMaskImage("linear-gradient(to right, transparent, rgba(0,0,0,0) 10px, black 40px)");
    } else {
      setMaskImage(
        "linear-gradient(to right, transparent, rgba(0,0,0,0) 10px, black 40px, black calc(100% - 40px), rgba(0,0,0,0) calc(100% - 10px), transparent)"
      );
    }
  }, []);

  useLayoutEffect(() => {
    const scrollToRight = () => {
      if (!scrollRef.current) return;
      scrollRef.current.scrollLeft = scrollRef.current.scrollWidth;
      checkScroll();
    };

    scrollToRight();
    window.addEventListener("resize", scrollToRight);
    return () => window.removeEventListener("resize", scrollToRight);
  }, [checkScroll, days.length]);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const observer = new ResizeObserver(() => checkScroll());
    observer.observe(el);
    return () => observer.disconnect();
  }, [checkScroll]);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const cols = 54;
    const gapPx = 4;
    const minCell = 14;

    const compute = () => {
      const container = scrollRef.current;
      if (!container) return;

      const containerStyle = window.getComputedStyle(container);
      const paddingLeft = parseFloat(containerStyle.paddingLeft || "0") || 0;
      const paddingRight = parseFloat(containerStyle.paddingRight || "0") || 0;
      const available = container.clientWidth - paddingLeft - paddingRight;
      if (!Number.isFinite(available) || available <= 0) return;

      const next = Math.max(minCell, (available - gapPx * (cols - 1)) / cols);
      setCellPx((prev) => (Math.abs(prev - next) < 0.25 ? prev : next));
    };

    compute();

    const observer = new ResizeObserver(() => compute());
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const isInsideGrid = useCallback((target: EventTarget | null) => {
    return target instanceof Node && !!scrollRef.current?.contains(target);
  }, []);

  const showTooltipFor = useCallback(
    (day: ActivityDay, element: HTMLElement, source: TooltipState["source"]) => {
      const rect = element.getBoundingClientRect();
      setTooltip({ day, x: rect.left + rect.width / 2, y: rect.top, source });
    },
    []
  );

  // 隐藏时区分来源：键盘焦点仍在网格内则不隐藏，避免鼠标离开误隐藏仍聚焦的格子 tooltip
  const hideTooltip = useCallback(
    (day: ActivityDay, source: TooltipState["source"], relatedTarget: EventTarget | null) => {
      setTooltip((prev) => {
        if (!prev || prev.source !== source || prev.day.dateStr !== day.dateStr) return prev;
        if (source === "focus" && isInsideGrid(relatedTarget)) return prev;
        return null;
      });
    },
    [isInsideGrid]
  );

  const renderTooltip = () => {
    if (!tooltip || typeof document === "undefined") return null;

    const isLeft = tooltip.x < 200;
    const isRight = tooltip.x > window.innerWidth - 200;
    const isTop = tooltip.y < window.innerHeight / 2;

    let transform = "translate(-50%, 15%)";
    if (!isTop && !isLeft && !isRight) {
      transform = "translate(-50%, -105%)";
    } else if (isTop && isLeft) {
      transform = "translate(10%, 15%)";
    } else if (isTop && isRight) {
      transform = "translate(-110%, 15%)";
    } else if (!isTop && isLeft) {
      transform = "translate(10%, -105%)";
    } else if (!isTop && isRight) {
      transform = "translate(-110%, -105%)";
    }

    const stat = tooltip.day.stat;
    const reqs = formatCompactCount(stat?.reqs);
    const tokens = formatCompactCount(stat?.tokens);

    return createPortal(
      <div
        className="fixed z-50 w-fit min-w-max text-sm bg-background text-foreground border rounded-3xl p-3 pointer-events-none"
        style={{
          left: tooltip.x,
          top: tooltip.y,
          transform,
        }}
      >
        <div className="space-y-2">
          <p className="font-semibold text-foreground">{tooltip.day.dateStr}</p>
          {stat ? (
            <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 items-center text-muted-foreground">
              {[
                { icon: MessageSquare, value: reqs.value, unit: reqs.unit },
                { icon: Bot, value: tokens.value, unit: tokens.unit },
              ].map((item, index) => (
                <Fragment key={index}>
                  <item.icon className="h-3.5 w-3.5" />
                  <span className="text-foreground font-medium text-right">
                    {item.value}
                    {item.unit}
                  </span>
                </Fragment>
              ))}
            </div>
          ) : (
            <div className="text-muted-foreground">-</div>
          )}
        </div>
      </div>,
      document.body
    );
  };

  const cardContent = (
    <div
      ref={scrollRef}
      onScroll={checkScroll}
      className="overflow-x-auto px-2 pb-2 pt-2"
      style={{ maskImage, WebkitMaskImage: maskImage }}
    >
      <div className="w-fit">
        <div
          className="grid"
          style={{
            gridTemplateColumns: "repeat(54, var(--cell))",
            gridTemplateRows: "repeat(7, var(--cell))",
            gridAutoFlow: "column",
            gap: "4px",
            "--cell": `${cellPx}px`,
          } as CSSProperties}
        >
          {days.map((day) => {
            if (day.isFuture) {
              return <div key={day.dateStr} />;
            }
            return (
              <HeatmapCell
                key={day.dateStr}
                day={day}
                onShowTooltip={showTooltipFor}
                onHideTooltip={hideTooltip}
              />
            );
          })}
        </div>
      </div>
    </div>
  );

  return (
    <div className="rounded-3xl bg-card border text-card-foreground custom-shadow p-4">
      <div className="mb-3 flex items-center gap-2.5">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Activity className="h-4 w-4" />
        </div>
        <div>
          <h3 className="text-base font-semibold leading-tight">活跃度</h3>
          <p className="text-xs text-muted-foreground sm:text-sm">近一年每日请求量分布</p>
        </div>
      </div>
      <div className="rounded-lg border bg-card p-3.5">
        {cardContent}
      </div>
      <div className="mt-3 flex items-center justify-end gap-1.5 px-2 text-xs text-muted-foreground">
        <span>少</span>
        {LEGEND_LEVELS.map((level) => (
          <div
            key={level}
            aria-hidden="true"
            className="h-3 w-3 rounded-sm ring-1 ring-border/20"
            style={{ backgroundColor: getLevelColor(level) }}
          />
        ))}
        <span>多</span>
      </div>
      {renderTooltip()}
    </div>
  );
}
