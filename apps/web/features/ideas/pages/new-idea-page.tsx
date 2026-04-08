"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, FilePlus2, Lightbulb, Sparkles } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button, buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { api } from "@/shared/api";
import type { IdeaNameSuggestion, IdeaOSConfig } from "@/shared/types";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

export function NewIdeaPage() {
  const router = useRouter();
  const [config, setConfig] = useState<IdeaOSConfig | null>(null);
  const [rawInput, setRawInput] = useState("");
  const [selectedName, setSelectedName] = useState("");
  const [nextCode, setNextCode] = useState("");
  const [suggestions, setSuggestions] = useState<IdeaNameSuggestion[]>([]);
  const [loadingConfig, setLoadingConfig] = useState(true);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoadingConfig(true);
      try {
        const nextConfig = await api.getIdeaOSConfig();
        if (!cancelled) {
          setConfig(nextConfig);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : "Failed to load IdeaOS config");
        }
      } finally {
        if (!cancelled) {
          setLoadingConfig(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!rawInput.trim() || !config?.github_connected) {
      setSuggestions([]);
      setNextCode("");
      return;
    }

    const timeout = window.setTimeout(async () => {
      setLoadingSuggestions(true);
      try {
        const resp = await api.recommendIdeaNames({ raw_input: rawInput.trim() });
        setSuggestions(resp.suggestions);
        setNextCode(resp.next_code);
        if (!selectedName && resp.suggestions[0]) {
          setSelectedName(resp.suggestions[0].name);
        }
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "Failed to generate name suggestions");
      } finally {
        setLoadingSuggestions(false);
      }
    }, 400);

    return () => window.clearTimeout(timeout);
  }, [config?.github_connected, rawInput, selectedName]);

  const configured = !!config?.repo_url && config?.github_connected;
  const slugPreview = useMemo(() => {
    const suggestion = suggestions.find((item) => item.name === selectedName);
    if (suggestion) return suggestion.full_name;
    if (!selectedName.trim() || !nextCode) return "";
    const suffix = selectedName
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
    return suffix ? `${nextCode}-${suffix}` : "";
  }, [nextCode, selectedName, suggestions]);

  const handleCreate = async () => {
    if (!rawInput.trim() || !selectedName.trim() || creating) return;
    setCreating(true);
    try {
      const idea = await api.createIdea({
        raw_input: rawInput.trim(),
        selected_name: selectedName.trim(),
      });
      router.push(`/ideas/${idea.slug}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create idea");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="flex h-full items-center justify-center px-6 py-8">
      <Card className="w-full max-w-3xl border-border/70 bg-background/90 shadow-sm">
        <CardHeader className="space-y-4">
          <Link href="/ideas" className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "w-fit -ml-2")}>
            <ArrowLeft className="h-4 w-4" />
            Back to Ideas
          </Link>
          <div className="space-y-3">
            <div className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary">
              <FilePlus2 className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="text-2xl">New Idea</CardTitle>
              <CardDescription className="mt-1">
                Describe the idea first. IdeaOS will suggest a final repository-safe name like <code>idea0007-your-idea</code>.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          {!configured && !loadingConfig ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-muted/30 p-4">
              <div className="flex items-start gap-3">
                <Lightbulb className="mt-0.5 h-4 w-4 text-primary" />
                <div className="space-y-2">
                  <p className="text-sm font-medium">IdeaOS is not configured yet</p>
                  <p className="text-sm text-muted-foreground">
                    Connect GitHub and configure your Ideas repository in workspace settings before creating ideas.
                  </p>
                  <Link href="/settings?tab=ideas" className={buttonVariants({ variant: "outline", size: "sm" })}>
                    Open IdeaOS Settings
                  </Link>
                </div>
              </div>
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <label htmlFor="idea-raw-input" className="text-sm font-medium">
                  Describe the idea
                </label>
                <Textarea
                  id="idea-raw-input"
                  value={rawInput}
                  onChange={(event) => setRawInput(event.target.value)}
                  rows={8}
                  placeholder="做一个 AI PR review agent，自动发现架构风险，并为每个 idea 自动创建项目代码仓库。"
                  className="resize-none"
                  autoFocus
                />
              </div>

              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4 text-primary" />
                  <p className="text-sm font-medium">Suggested names</p>
                  {loadingSuggestions && (
                    <span className="text-xs text-muted-foreground">Generating...</span>
                  )}
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  {suggestions.map((suggestion) => {
                    const selected = suggestion.name === selectedName;
                    return (
                      <button
                        key={suggestion.full_name}
                        type="button"
                        onClick={() => setSelectedName(suggestion.name)}
                        className={cn(
                          "rounded-xl border px-4 py-3 text-left transition-colors",
                          selected ? "border-primary bg-primary/5" : "border-border/70 hover:bg-muted/40"
                        )}
                      >
                        <div className="text-sm font-medium">{suggestion.name}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{suggestion.full_name}</div>
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="space-y-2">
                <label htmlFor="idea-selected-name" className="text-sm font-medium">
                  Final idea name
                </label>
                <Input
                  id="idea-selected-name"
                  value={selectedName}
                  onChange={(event) => setSelectedName(event.target.value)}
                  placeholder="AI PR Review"
                />
              </div>

              <div className="rounded-lg border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                {slugPreview ? (
                  <>
                    Directory & file: <code>{config?.directory || "ideas"}/{slugPreview}/{slugPreview}.md</code>
                  </>
                ) : (
                  "The final IdeaOS path preview will appear here."
                )}
              </div>

              <div className="flex flex-wrap gap-2">
                {config?.github_login && (
                  <Badge variant="secondary" className="rounded-full">
                    GitHub: {config.github_login}
                  </Badge>
                )}
                {config?.repo_url && (
                  <Badge variant="outline" className="rounded-full">
                    Ideas repo configured
                  </Badge>
                )}
              </div>

              <div className="flex justify-end">
                <Button onClick={handleCreate} disabled={!rawInput.trim() || !selectedName.trim() || creating}>
                  {creating ? "Creating..." : "Create"}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
