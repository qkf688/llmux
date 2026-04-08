import { describe, expect, it } from "vitest";
import { formatCompactCount, formatPercent, formatTimeNs } from "./formatters";

describe("formatCompactCount", () => {
  it("formats small numbers as integers", () => {
    expect(formatCompactCount(7)).toEqual({ raw: 7, value: "7", unit: "" });
    expect(formatCompactCount(0)).toEqual({ raw: 0, value: "0", unit: "" });
  });

  it("formats thousands with K", () => {
    expect(formatCompactCount(1234)).toEqual({ raw: 1234, value: "1.23", unit: "K" });
  });

  it("formats millions with M", () => {
    expect(formatCompactCount(1_500_000)).toEqual({ raw: 1_500_000, value: "1.50", unit: "M" });
  });

  it("formats billions with B", () => {
    expect(formatCompactCount(2_000_000_000)).toEqual({ raw: 2_000_000_000, value: "2.00", unit: "B" });
  });
});

describe("formatPercent", () => {
  it("formats ratio as percent", () => {
    expect(formatPercent(0.985, 1)).toBe("98.5%");
  });

  it("returns - for invalid input", () => {
    expect(formatPercent(undefined)).toBe("-");
    expect(formatPercent(Number.NaN)).toBe("-");
  });
});

describe("formatTimeNs", () => {
  it("formats nanoseconds into readable units", () => {
    expect(formatTimeNs(999)).toBe("999.00 ns");
    expect(formatTimeNs(1000)).toBe("1.00 μs");
    expect(formatTimeNs(1_000_000)).toBe("1.00 ms");
    expect(formatTimeNs(1_000_000_000)).toBe("1.00 s");
  });
});

