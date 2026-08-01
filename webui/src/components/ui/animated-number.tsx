import * as React from "react";
import { useReducedMotion } from "motion/react";
import { formatCompactCount } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { NUMBER_ANIMATION_MS } from "@/lib/animations/fluid-transitions";

type AnimatedNumberProps = {
  value: number;
  /** 动画时长（ms），默认走全局 token NUMBER_ANIMATION_MS */
  duration?: number;
  /** 自定义格式化；不传时默认用 formatCompactCount 紧凑格式 */
  formatter?: (value: number) => { value: string; unit?: string };
  /** 应用于数字容器的样式类 */
  className?: string;
};

// 数字滚动：从上一次可见值 lerp 到新值，避免 value 变化时回跳 0；
// rAF 副作用带 cleanup；系统"减少动态"时直接显示终值。
// displayRef 同步当前可见值，cleanup 时写回 fromRef，
// 保证动画被中途打断（value 在动画进行中变化）时下次从当前可见值继续。
export function AnimatedNumber({
  value,
  duration = NUMBER_ANIMATION_MS,
  formatter,
  className,
}: AnimatedNumberProps) {
  const [display, setDisplay] = React.useState(0);
  const fromRef = React.useRef(0);
  const displayRef = React.useRef(0);
  const rafRef = React.useRef<number | null>(null);
  const shouldReduceMotion = useReducedMotion();

  React.useEffect(() => {
    if (shouldReduceMotion) {
      setDisplay(value);
      fromRef.current = value;
      displayRef.current = value;
      return;
    }

    const from = fromRef.current;
    const to = value;
    let startTime: number | null = null;

    const tick = (timestamp: number) => {
      if (startTime === null) startTime = timestamp;
      const progress = Math.min((timestamp - startTime) / duration, 1);
      const current = from + (to - from) * progress;
      displayRef.current = current;
      setDisplay(current);
      if (progress < 1) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        fromRef.current = to;
      }
    };

    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      // 中断时把当前可见值作为下次起点，避免回跳到旧起点
      fromRef.current = displayRef.current;
    };
  }, [value, duration, shouldReduceMotion]);

  const format = formatter ?? ((v: number) => formatCompactCount(v));
  const { value: formattedValue, unit } = format(display);

  return (
    <div className={cn("flex items-baseline gap-1 tabular-nums", className)}>
      <span>{formattedValue}</span>
      {unit ? (
        <span className="text-xs text-muted-foreground leading-none">{unit}</span>
      ) : null}
    </div>
  );
}
