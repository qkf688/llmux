import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";

type CapturedProps = {
  initial?: unknown;
  animate?: unknown;
  exit?: unknown;
  children?: ReactNode;
};

const captured: CapturedProps[] = [];

// 捕获传给 motion.div 的 props，验证 staggered 是否按契约省略 initial/animate：
// motion 的 variant 传播会在自设 animate 的子级处终止，省略才能参与父级 staggerChildren 编排。
vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return {
    ...actual,
    useReducedMotion: () => false,
    motion: {
      div: (props: CapturedProps) => {
        captured.push(props);
        return <div>{props.children}</div>;
      },
    },
  };
});

import { AnimatedListItem } from "./animated-list-item";

describe("AnimatedListItem staggered", () => {
  it("默认自设 initial/animate（自驱入场）", () => {
    captured.length = 0;
    render(<AnimatedListItem>row</AnimatedListItem>);

    expect(captured).toHaveLength(1);
    expect(captured[0].initial).toBe("initial");
    expect(captured[0].animate).toBe("animate");
    expect(captured[0].exit).toBe("exit");
  });

  it("staggered 时省略 initial/animate 以继承父级编排，exit 保留供 AnimatePresence 使用", () => {
    captured.length = 0;
    render(<AnimatedListItem staggered>row</AnimatedListItem>);

    expect(captured).toHaveLength(1);
    expect(captured[0].initial).toBeUndefined();
    expect(captured[0].animate).toBeUndefined();
    expect(captured[0].exit).toBe("exit");
  });
});
