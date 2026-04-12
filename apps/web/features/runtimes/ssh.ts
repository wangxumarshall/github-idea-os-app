export type RuntimeSshConfig =
  | {
      enabled: true;
      host: string;
      port: number;
      user: string;
      reason: "";
    }
  | {
      enabled: false;
      host: string | null;
      port: number | null;
      user: string | null;
      reason: string;
    };

export function readRuntimeSSHConfig(metadata: Record<string, unknown>): RuntimeSshConfig {
  const enabled = metadata.ssh_enabled === true;
  const host = typeof metadata.ssh_host === "string"
    ? metadata.ssh_host.trim()
    : "";
  const user = typeof metadata.ssh_user === "string"
    ? metadata.ssh_user.trim()
    : "";

  let port = 22;
  if (typeof metadata.ssh_port === "number" && Number.isFinite(metadata.ssh_port)) {
    port = Math.trunc(metadata.ssh_port);
  } else if (typeof metadata.ssh_port === "string" && metadata.ssh_port.trim()) {
    const parsed = Number.parseInt(metadata.ssh_port.trim(), 10);
    if (!Number.isNaN(parsed) && parsed > 0) {
      port = parsed;
    }
  }

  if (!enabled) {
    return {
      enabled: false,
      host: host || null,
      port: host || user ? port : null,
      user: user || null,
      reason: "SSH access is not enabled for this runtime.",
    };
  }

  if (!host || !user) {
    return {
      enabled: false,
      host: host || null,
      port,
      user: user || null,
      reason: "SSH metadata is incomplete. Set ssh_host and ssh_user on the runtime.",
    };
  }

  return {
    enabled: true,
    host,
    port: port > 0 ? port : 22,
    user,
    reason: "",
  };
}

export function adminSshSessionStorageKey(runtimeId: string): string {
  return `multica_admin_ssh_session:${runtimeId}`;
}
