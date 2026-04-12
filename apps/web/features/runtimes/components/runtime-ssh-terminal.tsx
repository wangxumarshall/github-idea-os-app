"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  Loader2,
  MonitorCog,
  Plug2,
  RefreshCw,
  Shield,
  SquareTerminal,
  Unplug,
} from "lucide-react";
import { api } from "@/shared/api";
import type { AdminSshSession, AgentRuntime } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { adminSshSessionStorageKey, readRuntimeSSHConfig } from "../ssh";

type TerminalConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnected"
  | "closed"
  | "exited";

type XTermModule = typeof import("@xterm/xterm");
type FitAddonModule = typeof import("@xterm/addon-fit");
type TerminalInstance = InstanceType<XTermModule["Terminal"]>;
type FitAddonInstance = InstanceType<FitAddonModule["FitAddon"]>;

type AdminSshEvent = {
  type: string;
  data?: string;
  error?: string;
  exit_code?: number;
};

const DEFAULT_TMUX_SESSION_NAME = "multica-admin";

function resolveAdminSshWebSocketUrl(path: string, token: string): string {
  const base = process.env.NEXT_PUBLIC_API_URL
    ? new URL(process.env.NEXT_PUBLIC_API_URL, window.location.origin)
    : new URL(window.location.origin);
  const url = new URL(path, base);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("token", token);
  return url.toString();
}

function renderStatusLabel(state: TerminalConnectionState): string {
  switch (state) {
    case "connecting":
      return "Connecting";
    case "connected":
      return "Live";
    case "disconnected":
      return "Detached";
    case "closed":
      return "Closed";
    case "exited":
      return "Exited";
    default:
      return "Idle";
  }
}

export function RuntimeSshTerminal({ runtime }: { runtime: AgentRuntime }) {
  const user = useAuthStore((state) => state.user);
  const sshConfig = useMemo(
    () => readRuntimeSSHConfig(runtime.metadata),
    [runtime.metadata],
  );

  const terminalContainerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<TerminalInstance | null>(null);
  const fitAddonRef = useRef<FitAddonInstance | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const sessionRef = useRef<AdminSshSession | null>(null);

  const [terminalReady, setTerminalReady] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [session, setSession] = useState<AdminSshSession | null>(null);
  const [connectionState, setConnectionState] = useState<TerminalConnectionState>("idle");
  const [error, setError] = useState("");

  const storageKey = adminSshSessionStorageKey(runtime.id);

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  useEffect(() => {
    if (!user?.is_super_admin || !terminalContainerRef.current) {
      return;
    }

    let disposed = false;
    let dataSubscription: { dispose: () => void } | null = null;
    let resizeSubscription: { dispose: () => void } | null = null;

    const setup = async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      if (disposed || !terminalContainerRef.current) {
        return;
      }

      const terminal = new Terminal({
        cursorBlink: true,
        convertEol: true,
        fontFamily: "var(--font-mono)",
        fontSize: 13,
        lineHeight: 1.25,
        scrollback: 5000,
        theme: {
          background: "#08101d",
          foreground: "#d9e4ff",
          cursor: "#8bd3ff",
          cursorAccent: "#08101d",
          selectionBackground: "#25405f",
          black: "#101828",
          red: "#ff7b7b",
          green: "#7be0ad",
          yellow: "#f2cf65",
          blue: "#8bd3ff",
          magenta: "#d4a5ff",
          cyan: "#86efe7",
          white: "#eef4ff",
          brightBlack: "#52627d",
          brightRed: "#ff9b9b",
          brightGreen: "#93efbb",
          brightYellow: "#ffe082",
          brightBlue: "#a7e3ff",
          brightMagenta: "#dfb6ff",
          brightCyan: "#9cf7ef",
          brightWhite: "#ffffff",
        },
      });

      const fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);
      terminal.open(terminalContainerRef.current);
      fitAddon.fit();
      terminal.focus();
      terminal.writeln("\x1b[38;5;81mSSH control channel ready.\x1b[0m");

      dataSubscription = terminal.onData((data) => {
        if (socketRef.current?.readyState === WebSocket.OPEN) {
          socketRef.current.send(JSON.stringify({ type: "stdin", data }));
        }
      });

      resizeSubscription = terminal.onResize(({ cols, rows }) => {
        if (socketRef.current?.readyState === WebSocket.OPEN) {
          socketRef.current.send(JSON.stringify({ type: "resize", cols, rows }));
        }
      });

      if (typeof ResizeObserver !== "undefined") {
        const observer = new ResizeObserver(() => {
          fitAddon.fit();
          if (socketRef.current?.readyState === WebSocket.OPEN) {
            socketRef.current.send(JSON.stringify({
              type: "resize",
              cols: terminal.cols,
              rows: terminal.rows,
            }));
          }
        });
        observer.observe(terminalContainerRef.current);
        resizeObserverRef.current = observer;
      }

      terminalRef.current = terminal;
      fitAddonRef.current = fitAddon;
      setTerminalReady(true);
    };

    void setup();

    return () => {
      disposed = true;
      dataSubscription?.dispose();
      resizeSubscription?.dispose();
      resizeObserverRef.current?.disconnect();
      resizeObserverRef.current = null;
      socketRef.current?.close();
      socketRef.current = null;
      terminalRef.current?.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
      sessionRef.current = null;
      setTerminalReady(false);
    };
  }, [user?.is_super_admin]);

  const writeSystemLine = (text: string) => {
    terminalRef.current?.writeln(`\x1b[38;5;244m${text}\x1b[0m`);
  };

  const clearTerminal = () => {
    terminalRef.current?.clear();
    terminalRef.current?.write("\x1bc");
    writeSystemLine("Super admin SSH relay");
  };

  const disconnectSocket = () => {
    const socket = socketRef.current;
    socketRef.current = null;
    socket?.close();
  };

  const updateClosedSession = (reason: string, exitCode?: number) => {
    const nextState = reason.toLowerCase().includes("closed") ? "closed" : "exited";
    setConnectionState(nextState);
    setError(reason);
    setSession((current) =>
      current
        ? {
            ...current,
            status: nextState === "closed" ? "closed" : "exited",
            can_reconnect: false,
            exit_code: exitCode ?? current.exit_code ?? null,
            exit_error: reason,
            exited_at: current.exited_at ?? new Date().toISOString(),
          }
        : current,
    );
    localStorage.removeItem(storageKey);
  };

  const connectSocket = (nextSession: AdminSshSession) => {
    const token = localStorage.getItem("multica_token");
    if (!token) {
      throw new Error("Missing login token");
    }

    disconnectSocket();

    const socket = new WebSocket(
      resolveAdminSshWebSocketUrl(nextSession.websocket_path, token),
    );
    socketRef.current = socket;

    socket.onopen = () => {
      if (socketRef.current !== socket) return;
      setConnectionState("connected");
      setError("");
      writeSystemLine(`Connected to ${nextSession.user}@${nextSession.host}:${nextSession.port}`);
      fitAddonRef.current?.fit();
      socket.send(JSON.stringify({
        type: "resize",
        cols: terminalRef.current?.cols ?? 120,
        rows: terminalRef.current?.rows ?? 36,
      }));
      terminalRef.current?.focus();
    };

    socket.onmessage = (event) => {
      const payload = JSON.parse(event.data as string) as AdminSshEvent;

      switch (payload.type) {
        case "ready":
          return;
        case "output":
        case "stdout":
          if (payload.data) {
            terminalRef.current?.write(payload.data);
          }
          return;
        case "error":
          if (payload.error) {
            setError(payload.error);
            writeSystemLine(payload.error);
          }
          return;
        case "exit":
          updateClosedSession(payload.error ?? "SSH session exited", payload.exit_code);
          writeSystemLine(payload.error ?? "SSH session exited");
          disconnectSocket();
          return;
        default:
          return;
      }
    };

    socket.onclose = () => {
      if (socketRef.current !== socket) return;
      socketRef.current = null;
      setConnectionState((current) => {
        if (current === "closed" || current === "exited") return current;
        return sessionRef.current?.can_reconnect ? "disconnected" : "idle";
      });
    };

    socket.onerror = () => {
      if (socketRef.current !== socket) return;
      setError("SSH stream disconnected unexpectedly.");
    };
  };

  const connectToSession = (nextSession: AdminSshSession) => {
    setSession(nextSession);
    localStorage.setItem(storageKey, nextSession.id);
    connectSocket(nextSession);
  };

  useEffect(() => {
    if (!terminalReady) return;

    const storedSessionId = localStorage.getItem(storageKey);
    if (!storedSessionId) {
      setSession(null);
      setConnectionState("idle");
      return;
    }

    let cancelled = false;
    setRestoring(true);
    clearTerminal();
    writeSystemLine("Restoring previous SSH session...");

    void api.getAdminSshSession(storedSessionId)
      .then((storedSession) => {
        if (cancelled) return;
        setSession(storedSession);

        if (storedSession.can_reconnect) {
          setConnectionState("connecting");
          connectSocket(storedSession);
          return;
        }

        localStorage.removeItem(storageKey);
        setConnectionState(
          storedSession.status === "closed" ? "closed" : "exited",
        );
      })
      .catch(() => {
        if (cancelled) return;
        localStorage.removeItem(storageKey);
        setSession(null);
        setConnectionState("idle");
        clearTerminal();
      })
      .finally(() => {
        if (!cancelled) {
          setRestoring(false);
        }
      });

    return () => {
      cancelled = true;
      disconnectSocket();
    };
  }, [storageKey, terminalReady]);

  const handleConnect = async () => {
    if (!terminalReady || !sshConfig.enabled) {
      return;
    }

    clearTerminal();
    setError("");
    setConnectionState("connecting");

    try {
      const nextSession = await api.createAdminSshSession(runtime.id);
      connectToSession(nextSession);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to start SSH session";
      setError(message);
      setConnectionState("idle");
      writeSystemLine(message);
    }
  };

  const handleReconnect = async () => {
    if (!session) {
      await handleConnect();
      return;
    }

    clearTerminal();
    setError("");
    setConnectionState("connecting");

    try {
      const refreshed = await api.getAdminSshSession(session.id);
      connectToSession(refreshed);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to reconnect SSH session";
      setError(message);
      setConnectionState("idle");
      writeSystemLine(message);
      localStorage.removeItem(storageKey);
    }
  };

  const handleClose = async () => {
    if (!session) {
      disconnectSocket();
      setConnectionState("idle");
      localStorage.removeItem(storageKey);
      return;
    }

    try {
      const closed = await api.closeAdminSshSession(session.id);
      setSession(closed);
      setConnectionState("closed");
      setError(closed.exit_error ?? "");
      localStorage.removeItem(storageKey);
      disconnectSocket();
      writeSystemLine("Session closed by super admin.");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to close SSH session";
      setError(message);
      writeSystemLine(message);
    }
  };

  if (!user?.is_super_admin) {
    return null;
  }

  const statusLabel = renderStatusLabel(connectionState);
  const targetLabel = sshConfig.enabled
    ? `${sshConfig.user}@${sshConfig.host}:${sshConfig.port}`
    : runtime.name;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="text-xs font-medium text-muted-foreground">
            SSH Terminal
          </h3>
          <p className="mt-1 text-sm text-foreground">
            Super-admin relay with automatic tmux attach to{" "}
            <span className="font-mono text-xs">{DEFAULT_TMUX_SESSION_NAME}</span>.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge
            variant="outline"
            className={cn(
              "gap-1.5 border-white/10 bg-slate-950/50 px-2.5 text-slate-200",
              connectionState === "connected" && "border-cyan-400/30 text-cyan-200",
              connectionState === "connecting" && "border-amber-300/30 text-amber-100",
              (connectionState === "closed" || connectionState === "exited") && "border-rose-300/30 text-rose-100",
            )}
          >
            <MonitorCog className="h-3 w-3" />
            {restoring ? "Restoring" : statusLabel}
          </Badge>
          {(session || sshConfig.enabled) && (
            <Badge variant="outline" className="gap-1.5">
              <Shield className="h-3 w-3" />
              {session ? `${session.user}@${session.host}` : `${sshConfig.user}@${sshConfig.host}`}
            </Badge>
          )}
        </div>
      </div>

      {!sshConfig.enabled && (
        <Alert variant="destructive">
          <AlertTriangle />
          <AlertTitle>SSH metadata missing</AlertTitle>
          <AlertDescription>{sshConfig.reason}</AlertDescription>
        </Alert>
      )}

      {sshConfig.enabled && runtime.status !== "online" && (
        <Alert>
          <Unplug />
          <AlertTitle>Runtime offline</AlertTitle>
          <AlertDescription>
            SSH is configured for this runtime, but the runtime is currently offline.
          </AlertDescription>
        </Alert>
      )}

      <div className="rounded-[1.25rem] border bg-card shadow-sm">
        <div className="border-b px-4 py-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-2xl border border-cyan-300/25 bg-[linear-gradient(135deg,rgba(34,211,238,0.12),rgba(14,165,233,0.02))] text-cyan-600 dark:text-cyan-300">
                <SquareTerminal className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <div className="text-sm font-semibold text-foreground">
                  {sshConfig.enabled ? targetLabel : "Awaiting runtime SSH configuration"}
                </div>
                <div className="text-xs text-muted-foreground">
                  Host verification stays strict. Detached sessions expire after inactivity.
                </div>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  void navigator.clipboard?.writeText(targetLabel);
                }}
              >
                <Plug2 className="h-3.5 w-3.5" />
                Copy target
              </Button>

              {connectionState === "disconnected" && session ? (
                <Button size="sm" onClick={handleReconnect}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  Reconnect
                </Button>
              ) : (
                <Button
                  size="sm"
                  onClick={handleConnect}
                  disabled={
                    restoring ||
                    !terminalReady ||
                    !sshConfig.enabled ||
                    runtime.status !== "online" ||
                    connectionState === "connecting" ||
                    connectionState === "connected"
                  }
                >
                  {connectionState === "connecting" || restoring ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      Starting
                    </>
                  ) : (
                    <>
                      <Plug2 className="h-3.5 w-3.5" />
                      Connect
                    </>
                  )}
                </Button>
              )}

              <Button
                variant="ghost"
                size="sm"
                onClick={handleClose}
                disabled={!session || connectionState === "connecting"}
              >
                <Unplug className="h-3.5 w-3.5" />
                End session
              </Button>
            </div>
          </div>
        </div>

        <div className="px-4 pb-4 pt-3">
          <div className="grid gap-2 pb-3 text-xs text-muted-foreground sm:grid-cols-3">
            <div className="rounded-xl border bg-muted/40 px-3 py-2">
              <div className="font-medium text-foreground">SSH target</div>
              <div className="mt-1 font-mono">
                {sshConfig.enabled ? `${sshConfig.user}@${sshConfig.host}` : "Unavailable"}
              </div>
            </div>
            <div className="rounded-xl border bg-muted/40 px-3 py-2">
              <div className="font-medium text-foreground">tmux mode</div>
              <div className="mt-1 font-mono">
                {session?.tmux_session ?? DEFAULT_TMUX_SESSION_NAME}
              </div>
            </div>
            <div className="rounded-xl border bg-muted/40 px-3 py-2">
              <div className="font-medium text-foreground">Session</div>
              <div className="mt-1 font-mono text-[11px]">
                {session?.id ?? "Not started"}
              </div>
            </div>
          </div>

          <div className="runtime-ssh-terminal relative overflow-hidden rounded-[1.1rem] border border-slate-800/80 bg-[radial-gradient(circle_at_top,rgba(34,211,238,0.08),transparent_34%),linear-gradient(180deg,#0f172a_0%,#07111f_100%)] shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]">
            <div className="flex items-center justify-between border-b border-white/8 px-3 py-2 text-[11px] uppercase tracking-[0.18em] text-slate-400">
              <span>Remote shell</span>
              <span>{restoring ? "Restoring" : statusLabel}</span>
            </div>
            <div
              ref={terminalContainerRef}
              className="min-h-[360px] w-full"
            />
          </div>

          {error && (
            <p className="mt-3 text-sm text-destructive">{error}</p>
          )}
        </div>
      </div>
    </div>
  );
}
