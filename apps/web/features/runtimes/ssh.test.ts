import { describe, expect, it } from "vitest";
import {
  adminSshSessionStorageKey,
  readRuntimeSSHConfig,
  TERMINAL_SCROLLBACK_LINES,
} from "./ssh";

describe("readRuntimeSSHConfig", () => {
  it("returns disabled when ssh is not enabled", () => {
    expect(readRuntimeSSHConfig({})).toEqual({
      enabled: false,
      host: null,
      port: null,
      user: null,
      reason: "SSH access is not enabled for this runtime.",
    });
  });

  it("parses enabled SSH metadata with default port", () => {
    expect(
      readRuntimeSSHConfig({
        ssh_enabled: true,
        ssh_host: "devbox.internal",
        ssh_user: "ubuntu",
      }),
    ).toEqual({
      enabled: true,
      host: "devbox.internal",
      port: 22,
      user: "ubuntu",
      reason: "",
    });
  });

  it("rejects incomplete SSH metadata", () => {
    expect(
      readRuntimeSSHConfig({
        ssh_enabled: true,
        ssh_host: "devbox.internal",
      }),
    ).toEqual({
      enabled: false,
      host: "devbox.internal",
      port: 22,
      user: null,
      reason: "SSH metadata is incomplete. Set ssh_host and ssh_user on the runtime.",
    });
  });
});

describe("adminSshSessionStorageKey", () => {
  it("uses a stable local storage key per runtime", () => {
    expect(adminSshSessionStorageKey("runtime-123")).toBe(
      "multica_admin_ssh_session:runtime-123",
    );
  });
});

describe("TERMINAL_SCROLLBACK_LINES", () => {
  it("uses an extended scrollback buffer for SSH sessions", () => {
    expect(TERMINAL_SCROLLBACK_LINES).toBe(50000);
  });
});
