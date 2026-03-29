import { describe, expect, it } from "vitest";
import { toErrorMessage } from "./errors";

describe("toErrorMessage", () => {
  it("returns error.message for Error", () => {
    expect(toErrorMessage(new Error("boom"))).toBe("boom");
  });

  it("returns fallback for empty Error message", () => {
    expect(toErrorMessage(new Error(""), "x")).toBe("x");
  });

  it("returns trimmed string for string errors", () => {
    expect(toErrorMessage("  hi  ")).toBe("hi");
  });

  it("returns fallback for empty string errors", () => {
    expect(toErrorMessage("   ", "x")).toBe("x");
  });

  it("returns stringified primitives", () => {
    expect(toErrorMessage(0)).toBe("0");
    expect(toErrorMessage(true)).toBe("true");
  });

  it("returns message field from objects when present", () => {
    expect(toErrorMessage({ message: "nope" })).toBe("nope");
    expect(toErrorMessage({ message: "  ok " })).toBe("ok");
  });

  it("returns fallback for objects without message", () => {
    expect(toErrorMessage({ code: "E" }, "x")).toBe("x");
  });
});

