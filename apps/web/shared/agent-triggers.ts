import type { AgentTrigger, AgentTriggerType } from "@/shared/types";

export const AGENT_TRIGGER_TYPES = [
  "on_assign",
  "on_comment",
  "on_mention",
  "on_scheduled",
] as const satisfies AgentTriggerType[];

export const DEFAULT_SCHEDULED_TRIGGER_CONFIG = {
  cron: "0 9 * * 1-5",
  timezone: "UTC",
} satisfies Record<string, string>;

export interface NormalizedAgentTrigger {
  id: string;
  type: AgentTriggerType;
  enabled: boolean;
  config: Record<string, unknown>;
}

export function normalizeTriggerConfig(
  type: AgentTriggerType,
  config: Record<string, unknown> | null | undefined,
): Record<string, unknown> {
  const normalized = { ...(config ?? {}) };
  if (type === "on_scheduled") {
    if (typeof normalized.cron !== "string" || normalized.cron.trim() === "") {
      normalized.cron = DEFAULT_SCHEDULED_TRIGGER_CONFIG.cron;
    }
    if (typeof normalized.timezone !== "string" || normalized.timezone.trim() === "") {
      normalized.timezone = DEFAULT_SCHEDULED_TRIGGER_CONFIG.timezone;
    }
  }
  return normalized;
}

function isAgentTriggerType(value: string): value is AgentTriggerType {
  return (AGENT_TRIGGER_TYPES as readonly string[]).includes(value);
}

export function normalizeAgentTriggers(rawTriggers: AgentTrigger[] | null | undefined): NormalizedAgentTrigger[] {
  const normalized = (rawTriggers ?? [])
    .map((trigger, index) => {
      const type = typeof trigger.type === "string" ? trigger.type.trim() : "";
      if (!isAgentTriggerType(type)) {
        return null;
      }
      return {
        id: trigger.id?.trim() || `${type}-${index}`,
        type,
        enabled: trigger.enabled ?? true,
        config: normalizeTriggerConfig(type, trigger.config),
      } satisfies NormalizedAgentTrigger;
    })
    .filter((trigger): trigger is NormalizedAgentTrigger => trigger !== null);

  if (normalized.length === 0) {
    return defaultAgentTriggers();
  }

  return normalized;
}

export function defaultAgentTriggers(): NormalizedAgentTrigger[] {
  return [
    {
      id: "on_assign-default",
      type: "on_assign",
      enabled: true,
      config: {},
    },
    {
      id: "on_comment-default",
      type: "on_comment",
      enabled: true,
      config: {},
    },
    {
      id: "on_mention-default",
      type: "on_mention",
      enabled: true,
      config: {},
    },
    {
      id: "on_scheduled-default",
      type: "on_scheduled",
      enabled: false,
      config: { ...DEFAULT_SCHEDULED_TRIGGER_CONFIG },
    },
  ];
}
