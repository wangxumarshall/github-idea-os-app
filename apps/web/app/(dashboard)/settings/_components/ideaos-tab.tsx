"use client";

import { useEffect, useState } from "react";
import { FolderGit2, Link2, Save, Sparkles, Unplug } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { toast } from "sonner";
import { useSearchParams } from "next/navigation";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import type { IdeaOSConfig } from "@/shared/types";

const defaultConfig: IdeaOSConfig = {
  repo_url: "",
  branch: "main",
  directory: "ideas",
  repo_visibility: "private",
  default_agent_ids: [],
  github_connected: false,
  github_login: null,
};

function normalizeIdeaOSConfig(config: Partial<IdeaOSConfig> | null | undefined): IdeaOSConfig {
  return {
    ...defaultConfig,
    ...config,
    default_agent_ids: Array.isArray(config?.default_agent_ids) ? config.default_agent_ids : [],
  };
}

export function IdeaOSTab() {
  const searchParams = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const [config, setConfig] = useState<IdeaOSConfig>(defaultConfig);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);

  const currentMember = members.find((member) => member.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const availableAgents = agents.filter((agent) => !agent.archived_at);

  useEffect(() => {
    if (!workspace) return;
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      try {
        const nextConfig = await api.getIdeaOSConfig();
        if (!cancelled) {
          setConfig(normalizeIdeaOSConfig(nextConfig));
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : "Failed to load IdeaOS config");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [workspace]);

  useEffect(() => {
    const githubStatus = searchParams.get("github");
    if (githubStatus === "connected") {
      toast.success("GitHub connected");
    } else if (githubStatus === "error") {
      toast.error("GitHub connection failed");
    }
  }, [searchParams]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const nextConfig = await api.updateIdeaOSConfig({
        repo_url: config.repo_url,
        branch: config.branch,
        directory: config.directory,
        repo_visibility: config.repo_visibility,
        default_agent_ids: config.default_agent_ids,
      });
      setConfig(normalizeIdeaOSConfig(nextConfig));
      toast.success("IdeaOS settings saved");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to save IdeaOS settings");
    } finally {
      setSaving(false);
    }
  };

  const handleConnectGitHub = async () => {
    setConnecting(true);
    try {
      const resp = await api.startGitHubOAuth("/settings?tab=ideas&github=connected");
      window.location.href = resp.authorize_url;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to start GitHub OAuth");
      setConnecting(false);
    }
  };

  const handleDisconnectGitHub = async () => {
    setDisconnecting(true);
    try {
      await api.disconnectGitHubAccount();
      const nextConfig = await api.getIdeaOSConfig();
      setConfig(normalizeIdeaOSConfig(nextConfig));
      toast.success("GitHub disconnected");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to disconnect GitHub");
    } finally {
      setDisconnecting(false);
    }
  };

  if (!workspace) return null;

  const toggleDefaultAgent = (agentId: string) => {
    setConfig((current) => ({
      ...current,
      default_agent_ids: current.default_agent_ids.includes(agentId)
        ? current.default_agent_ids.filter((id) => id !== agentId)
        : [...current.default_agent_ids, agentId],
    }));
  };

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <div className="space-y-2">
          <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-background px-3 py-1 text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">
            <Sparkles className="h-3.5 w-3.5" />
            GitHub IdeaOS
          </div>
          <div>
            <h2 className="text-sm font-semibold">Ideas</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Connect one GitHub repository and use it as the source of truth for versioned product and technical ideas.
            </p>
          </div>
        </div>

        <Card>
          <CardContent className="space-y-4">
            <div className="rounded-xl border border-border/60 bg-muted/20 p-4 text-sm text-muted-foreground">
              Every idea is stored as <code>ideas/idea0001-name/idea0001-name.md</code>. The app writes markdown updates to your Ideas repository and provisions a matching private project repository in your GitHub account.
            </div>

            <div className="rounded-xl border border-border/60 bg-background/60 p-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <FolderGit2 className="h-4 w-4 text-muted-foreground" />
                    <p className="text-sm font-medium">GitHub account</p>
                    <Badge variant={config.github_connected ? "secondary" : "outline"} className="rounded-full">
                      {config.github_connected ? "Connected" : "Not connected"}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {config.github_connected && config.github_login
                      ? `Connected as ${config.github_login}. New project repositories will be created in this account.`
                      : "Connect GitHub before creating ideas or provisioning repositories."}
                  </p>
                </div>
                <div className="flex gap-2">
                  {config.github_connected ? (
                    <Button variant="outline" size="sm" onClick={handleDisconnectGitHub} disabled={!canManageWorkspace || disconnecting || loading}>
                      <Unplug className="h-3.5 w-3.5" />
                      {disconnecting ? "Disconnecting..." : "Disconnect"}
                    </Button>
                  ) : (
                    <Button size="sm" onClick={handleConnectGitHub} disabled={!canManageWorkspace || connecting || loading}>
                      <Link2 className="h-3.5 w-3.5" />
                      {connecting ? "Connecting..." : "Connect GitHub"}
                    </Button>
                  )}
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="ideaos-repo" className="text-xs text-muted-foreground">Repository URL</Label>
              <div className="relative">
                <FolderGit2 className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="ideaos-repo"
                  value={config.repo_url}
                  onChange={(event) => setConfig((current) => ({ ...current, repo_url: event.target.value }))}
                  disabled={!canManageWorkspace || loading}
                  placeholder="https://github.com/org/ideas"
                  className="pl-9"
                />
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="ideaos-branch" className="text-xs text-muted-foreground">Branch</Label>
                <Input
                  id="ideaos-branch"
                  value={config.branch}
                  onChange={(event) => setConfig((current) => ({ ...current, branch: event.target.value }))}
                  disabled={!canManageWorkspace || loading}
                  placeholder="main"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="ideaos-directory" className="text-xs text-muted-foreground">Directory</Label>
                <Input
                  id="ideaos-directory"
                  value={config.directory}
                  onChange={(event) => setConfig((current) => ({ ...current, directory: event.target.value }))}
                  disabled={!canManageWorkspace || loading}
                  placeholder="ideas"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="ideaos-visibility" className="text-xs text-muted-foreground">Project repository visibility</Label>
              <div className="flex gap-2">
                {(["private", "public"] as const).map((visibility) => (
                  <Button
                    key={visibility}
                    type="button"
                    size="sm"
                    variant={config.repo_visibility === visibility ? "default" : "outline"}
                    disabled={!canManageWorkspace || loading}
                    onClick={() => setConfig((current) => ({ ...current, repo_visibility: visibility }))}
                  >
                    {visibility}
                  </Button>
                ))}
              </div>
            </div>

            <div className="space-y-3">
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">Default idea agents</Label>
                <p className="text-sm text-muted-foreground">
                  When selected, the first valid agent in this list will be assigned to the root issue created for a new idea. If none are selected, the app only creates the issue and does not auto-dispatch work.
                </p>
              </div>

              <div className="space-y-2 rounded-xl border border-border/60 bg-background/40 p-3">
                {availableAgents.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No active agents available in this workspace.</p>
                ) : (
                  availableAgents.map((agent) => {
                    const checked = config.default_agent_ids.includes(agent.id);
                    return (
                      <label key={agent.id} className="flex items-start gap-3 rounded-lg px-2 py-2 text-sm hover:bg-accent/30">
                        <Checkbox
                          checked={checked}
                          disabled={!canManageWorkspace || loading}
                          onCheckedChange={() => toggleDefaultAgent(agent.id)}
                        />
                        <div className="space-y-1">
                          <div className="font-medium">{agent.name}</div>
                          <div className="text-xs text-muted-foreground">
                            {agent.description || "No description"}
                          </div>
                        </div>
                      </label>
                    );
                  })
                )}
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 pt-2">
              <Button size="sm" onClick={handleSave} disabled={!canManageWorkspace || saving || loading || !config.repo_url.trim()}>
                <Save className="h-3 w-3" />
                {saving ? "Saving..." : "Save"}
              </Button>
            </div>

            {!canManageWorkspace && (
              <p className="text-xs text-muted-foreground">
                Only admins and owners can update IdeaOS settings.
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
