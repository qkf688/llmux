import { describe, expect, it } from "vitest";

import {
  HEALTH_CHECK_STATUS_BADGE_CLASS,
  HEALTH_CHECK_STATUS_TEXT_CLASS,
  HEALTH_CHECK_STATUSES,
  getHealthCheckStatusLabel,
} from "./health-check-status";

describe("health-check-status", () => {
  it("三态文案与后端约定逐一对应", () => {
    expect(HEALTH_CHECK_STATUSES).toEqual(["success", "error", "not_measured"]);
    expect(getHealthCheckStatusLabel("success")).toBe("成功");
    expect(getHealthCheckStatusLabel("error")).toBe("失败");
    expect(getHealthCheckStatusLabel("not_measured")).toBe("未完成");
  });

  it("每个状态都备齐文字与徽标配色（漏配会让该态静默无色）", () => {
    for (const status of HEALTH_CHECK_STATUSES) {
      expect(HEALTH_CHECK_STATUS_TEXT_CLASS[status]).toBeTruthy();
      expect(HEALTH_CHECK_STATUS_BADGE_CLASS[status]).toBeTruthy();
    }
  });
});
