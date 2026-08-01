import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { Suspense } from "react";
import { useLocation, useOutlet } from "react-router-dom";
import Loading from "@/components/loading";
import { EASING } from "@/lib/animations/fluid-transitions";

function ContentFallback() {
  return (
    <div className="flex items-center justify-center py-10">
      <Loading message="加载页面" />
    </div>
  );
}

export function AnimatedOutlet() {
  const location = useLocation();
  const outlet = useOutlet();
  // 与 home / AnimatedNumber 统一走 motion 的 useReducedMotion（静态快照，运行中切换系统设置不实时响应）
  const prefersReducedMotion = useReducedMotion();

  const transition = prefersReducedMotion
    ? { duration: 0 }
    : { duration: 0.18, ease: EASING.easeOutExpo };
  const initial = prefersReducedMotion ? { opacity: 1, y: 0 } : { opacity: 0, y: 6 };
  const animate = { opacity: 1, y: 0 };
  const exit = prefersReducedMotion ? { opacity: 1, y: 0 } : { opacity: 0, y: -6 };

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={location.pathname}
        initial={initial}
        animate={animate}
        exit={exit}
        transition={transition}
        className="h-full min-w-0"
      >
        <Suspense fallback={<ContentFallback />}>{outlet}</Suspense>
      </motion.div>
    </AnimatePresence>
  );
}
