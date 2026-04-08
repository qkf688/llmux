"use client";

import { Fragment, useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Bot, MessageSquare } from "lucide-react";
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

function getActivityLevel(value: number): number {
  if (value <= 0) return 0;
  return LEVELS.find((l) => value >= l.min)?.level ?? 1;
}

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
  const [tooltip, setTooltip] = useState<{ day: ActivityDay; x: number; y: number; visible: boolean } | null>(null);
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
        className={`fixed z-50 w-fit min-w-max text-sm bg-background text-foreground border rounded-3xl p-3 transition-opacity duration-500 pointer-events-none ${tooltip.visible ? "opacity-100" : "opacity-0"}`}
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
            ["--cell" as any]: `${cellPx}px`,
          }}
        >
          {days.map((day) => {
            if (day.isFuture) {
              return <div key={day.dateStr} />;
            }

            const rawValue = day.stat?.reqs ?? 0;
            const level = getActivityLevel(rawValue);
            const backgroundColor =
              level === 0 ? "var(--muted)" : `color-mix(in oklch, var(--primary) ${level * 25}%, var(--muted))`;

            return (
              <div
                key={day.dateStr}
                className="rounded-sm transition-all cursor-pointer hover:scale-150"
                onMouseEnter={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  setTooltip({ day, x: rect.left + rect.width / 2, y: rect.top, visible: true });
                }}
                onMouseLeave={() => setTooltip((prev) => (prev ? { ...prev, visible: false } : null))}
                style={{ backgroundColor }}
              />
            );
          })}
        </div>
      </div>
    </div>
  );

  return (
    <div className="rounded-3xl bg-card border text-card-foreground custom-shadow">
      {cardContent}
      {renderTooltip()}
    </div>
  );
}
