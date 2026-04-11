import { describe, expect, it } from "vitest";
import {
  defaultAgentTriggers,
  normalizeAgentTriggers,
} from "./agent-triggers";

describe("agent trigger helpers", () => {
  it("normalizes legacy on_mention triggers with null config and missing ids", () => {
    const normalized = normalizeAgentTriggers([
      {
        type: "on_assign",
        enabled: true,
        config: null,
      },
      {
        type: "on_comment",
        enabled: true,
        config: null,
      },
      {
        type: "on_mention",
        enabled: true,
        config: null,
      },
    ]);

    expect(normalized).toHaveLength(3);
    expect(normalized[2]).toMatchObject({
      id: "on_mention-2",
      type: "on_mention",
      enabled: true,
      config: {},
    });
  });

  it("defaults empty trigger lists to the full canonical trigger set", () => {
    const normalized = normalizeAgentTriggers([]);
    expect(normalized).toHaveLength(4);
    expect(normalized.map((trigger) => trigger.type)).toEqual([
      "on_assign",
      "on_comment",
      "on_mention",
      "on_scheduled",
    ]);
    expect(normalized[3]).toMatchObject({
      type: "on_scheduled",
      enabled: false,
      config: {
        cron: "0 9 * * 1-5",
        timezone: "UTC",
      },
    });
  });

  it("builds the default trigger payload with scheduled disabled", () => {
    expect(defaultAgentTriggers()).toMatchObject([
      { type: "on_assign", enabled: true },
      { type: "on_comment", enabled: true },
      { type: "on_mention", enabled: true },
      { type: "on_scheduled", enabled: false },
    ]);
  });
});
