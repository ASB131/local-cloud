import { describe, expect, it } from "vitest";
import { stepGridSize } from "./explorerGridSize";

describe("stepGridSize", () => {
  it("zooms in and out", () => {
    expect(stepGridSize("md", 1)).toBe("lg");
    expect(stepGridSize("md", -1)).toBe("sm");
    expect(stepGridSize("sm", -1)).toBe("sm");
    expect(stepGridSize("lg", 1)).toBe("lg");
  });
});
