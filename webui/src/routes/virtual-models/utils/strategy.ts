import type { VirtualModelStrategy } from "../types";

const strategyLabelMap: Record<VirtualModelStrategy, string> = {
  priority: "优先级+权重",
  round_robin: "轮询",
  random: "随机",
};

export function getStrategyLabel(strategy: string): string {
  return strategyLabelMap[strategy as VirtualModelStrategy] ?? strategy;
}
