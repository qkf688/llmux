import { describe, expect, it } from "vitest";
import { addDays, formatDateYYYYMMDD, startOfDay } from "./date";

describe("formatDateYYYYMMDD", () => {
  it("pads month and day with leading zeros", () => {
    expect(formatDateYYYYMMDD(new Date(2026, 0, 1))).toBe("2026-01-01");
    expect(formatDateYYYYMMDD(new Date(2026, 2, 9))).toBe("2026-03-09");
  });

  it("does not pad two-digit month and day", () => {
    expect(formatDateYYYYMMDD(new Date(2026, 11, 31))).toBe("2026-12-31");
  });

  it("handles month rollover from December to January", () => {
    expect(formatDateYYYYMMDD(new Date(2026, 11, 32))).toBe("2027-01-01");
  });
});

describe("addDays", () => {
  it("adds positive days within the same month", () => {
    expect(addDays(new Date(2026, 0, 1), 5)).toEqual(new Date(2026, 0, 6));
  });

  it("rolls over to the next month", () => {
    expect(addDays(new Date(2026, 0, 31), 1)).toEqual(new Date(2026, 1, 1));
  });

  it("rolls over to the next year", () => {
    expect(addDays(new Date(2026, 11, 31), 1)).toEqual(new Date(2027, 0, 1));
  });

  it("subtracts days with negative argument", () => {
    expect(addDays(new Date(2026, 1, 1), -1)).toEqual(new Date(2026, 0, 31));
    expect(addDays(new Date(2026, 0, 1), -1)).toEqual(new Date(2025, 11, 31));
  });

  it("does not mutate the input date", () => {
    const input = new Date(2026, 5, 15);
    const result = addDays(input, 10);
    expect(input).toEqual(new Date(2026, 5, 15));
    expect(result).not.toBe(input);
  });
});

describe("startOfDay", () => {
  it("zeroes out hours, minutes, seconds, milliseconds", () => {
    expect(startOfDay(new Date(2026, 5, 15, 23, 59, 59, 999))).toEqual(
      new Date(2026, 5, 15, 0, 0, 0, 0)
    );
  });

  it("does not mutate the input date", () => {
    const input = new Date(2026, 5, 15, 12, 30, 45);
    const result = startOfDay(input);
    expect(input).toEqual(new Date(2026, 5, 15, 12, 30, 45));
    expect(result).not.toBe(input);
  });
});
