import { describe, expect, it } from "vitest";
import {
  buildConfigWithAllModels,
  buildConfigWithModels,
  parseAllModelsFromConfig,
  parseCustomModelsFromConfig,
  parseUpstreamModelsFromConfig,
} from "./provider-models";

describe("provider-models config helpers", () => {
  it("parses upstream/custom models and dedupes", () => {
    const config = JSON.stringify({
      upstream_models: [" gpt-4 ", "gpt-4"],
      custom_models: ["a", 1, null, "b"],
    });

    expect(parseUpstreamModelsFromConfig(config)).toEqual(["gpt-4", "gpt-4"]);
    expect(parseCustomModelsFromConfig(config)).toEqual(["a", "b"]);
    expect(parseAllModelsFromConfig(config).sort()).toEqual(["a", "b", "gpt-4"].sort());
  });

  it("returns empty arrays for invalid config", () => {
    expect(parseAllModelsFromConfig("not-json")).toEqual([]);
    expect(parseUpstreamModelsFromConfig("not-json")).toEqual([]);
    expect(parseCustomModelsFromConfig("not-json")).toEqual([]);
  });

  it("buildConfigWithModels merges with existing keys", () => {
    const config = JSON.stringify({ x: 1, upstream_models: ["u1"] });
    const next = buildConfigWithModels(config, [" u1 ", "u2"], ["c1", "c1"]);
    expect(JSON.parse(next)).toEqual({
      x: 1,
      upstream_models: ["u1", "u2"],
      custom_models: ["c1"],
    });
  });

  it("buildConfigWithAllModels keeps upstream and moves new to custom", () => {
    const config = JSON.stringify({ upstream_models: ["u1"] });
    const next = buildConfigWithAllModels(config, ["u1", "c1", "c2"]);
    expect(JSON.parse(next)).toEqual({ upstream_models: ["u1"], custom_models: ["c1", "c2"] });
  });
});

