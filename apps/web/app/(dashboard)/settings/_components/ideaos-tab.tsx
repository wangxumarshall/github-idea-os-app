"use client";

import { useEffect, useState } from "react";
import { FolderGit2, KeyRound, Save, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import type { IdeaOSConfig } from "@/shared/types";

const defaultConfig: IdeaOSConfig = {
  repo_url: "",
  branch: "main",
  directory: "ideas",
  token_configured: false,
};

export function IdeaOSTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const [config, setConfig] = useState<IdeaOSConfig>(defaultConfig);
  const [githubToken, setGitHubToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const currentMember = members.find((member) => member.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";

  useEffect(() => {
    if (!workspace) return;
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      try {
        const nextConfig = await api.getIdeaOSConfig();
        if (!cancelled) {
          setConfig(nextConfig);
          setGitHubToken("");
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

  const handleSave = async () => {
    setSaving(true);
    try {
      const nextConfig = await api.updateIdeaOSConfig({
        repo_url: config.repo_url,
        branch: config.branch,
        directory: config.directory,
        ...(githubToken.trim() ? { github_token: githubToken.trim() } : {}),
      });
      setConfig(nextConfig);
      setGitHubToken("");
      toast.success("IdeaOS settings saved");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to save IdeaOS settings");
    } finally {
      setSaving(false);
    }
  };

  if (!workspace) return null;

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
              The MVP stores every idea as a markdown file inside a single GitHub directory, usually <code>ideas/</code>. Autosave in the editor writes directly through the GitHub Contents API.
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
              <Label htmlFor="ideaos-token" className="text-xs text-muted-foreground">GitHub token</Label>
              <div className="relative">
                <KeyRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="ideaos-token"
                  type="password"
                  value={githubToken}
                  onChange={(event) => setGitHubToken(event.target.value)}
                  disabled={!canManageWorkspace || loading}
                  placeholder={config.token_configured ? "Token already configured. Enter a new one to rotate." : "ghp_..."}
                  className="pl-9"
                />
              </div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Badge variant={config.token_configured ? "secondary" : "outline"} className="rounded-full">
                  {config.token_configured ? "Token configured" : "No token"}
                </Badge>
                The token is stored server-side and never returned to the browser after save.
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
