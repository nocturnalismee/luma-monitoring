import { describe, expect, it } from "vitest";
import { formatVersion } from "./version";

describe("formatVersion", () => {
  it("formats major and minor", () => {
    expect(formatVersion(1, 4)).toBe("v1.4");
  });

  it("handles zero components", () => {
    expect(formatVersion(0, 0)).toBe("v0.0");
  });
});
