import { AnimatePresence, motion } from "motion/react";
import { Suspense, useEffect, useState } from "react";
import { useLocation, useOutlet } from "react-router-dom";
import Loading from "@/components/loading";

function usePrefersReducedMotion() {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    const handleChange = () => setPrefersReducedMotion(mediaQuery.matches);

    handleChange();

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    }

    mediaQuery.addListener(handleChange);
    return () => mediaQuery.removeListener(handleChange);
  }, []);

  return prefersReducedMotion;
}

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
  const prefersReducedMotion = usePrefersReducedMotion();

  const transition = prefersReducedMotion
    ? { duration: 0 }
    : { duration: 0.18, ease: [0.2, 0, 0, 1] as const };
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
