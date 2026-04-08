"use client";

import Link from "next/link";
import { useDeferredValue, useEffect, useMemo, useState } from "react";
import { FolderGit2, Lightbulb, Plus, Search, Sparkles } from "lucide-react";
import { api } from "@/shared/api";
import type { IdeaOSConfig, IdeaSummary } from "@/shared/types";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

function IdeaCard({ idea }: { idea: IdeaSummary }) {
  return (
    <Link href={`/ideas/${idea.slug}`} className="block">
      <Card className="h-full border-border/70 bg-card/80 transition-colors hover:border-foreground/20 hover:bg-card">
        <CardHeader className="space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">{idea.code}</div>
              <CardTitle className="mt-1 text-base leading-tight">{idea.title}</CardTitle>
            </div>
            <span className="shrink-0 text-xs text-muted-foreground">{idea.updated_at}</span>
          </div>
          <CardDescription className="line-clamp-2 text-sm">
            {idea.summary || "No summary yet."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {idea.tags.length > 0 ? (
            idea.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="rounded-full px-2.5 py-0.5 text-[11px]">
                {tag}
              </Badge>
            ))
          ) : (
            <span className="text-xs text-muted-foreground">No tags</span>
          )}
          <Badge
            variant={idea.project_repo_status === "ready" ? "secondary" : idea.project_repo_status === "failed" ? "destructive" : "outline"}
            className="rounded-full px-2.5 py-0.5 text-[11px]"
          >
            <FolderGit2 className="h-3 w-3" />
            {idea.project_repo_status}
          </Badge>
        </CardContent>
      </Card>
    </Link>
  );
}

function IdeasPageSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {Array.from({ length: 6 }).map((_, index) => (
        <Card key={index} className="border-border/70">
          <CardHeader className="space-y-3">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-4/5" />
          </CardHeader>
          <CardContent className="flex gap-2">
            <Skeleton className="h-6 w-16 rounded-full" />
            <Skeleton className="h-6 w-20 rounded-full" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

export function IdeasPage() {
  const [config, setConfig] = useState<IdeaOSConfig | null>(null);
  const [ideas, setIdeas] = useState<IdeaSummary[]>([]);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      try {
        const [nextConfig, nextIdeas] = await Promise.all([
          api.getIdeaOSConfig(),
          api.listIdeas().catch((error) => {
            if (error instanceof Error && error.message.includes("not configured")) {
              return [] as IdeaSummary[];
            }
            throw error;
          }),
        ]);
        if (!cancelled) {
          setConfig(nextConfig);
          setIdeas(nextIdeas);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : "Failed to load ideas");
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
  }, []);

  const filteredIdeas = useMemo(() => {
    const needle = deferredSearch.trim().toLowerCase();
    if (!needle) return ideas;
    return ideas.filter((idea) =>
      [idea.title, idea.summary, idea.tags.join(" ")].some((value) =>
        value.toLowerCase().includes(needle)
      )
    );
  }, [ideas, deferredSearch]);

  const configured = !!config?.repo_url && config?.github_connected;

  return (
    <div className="flex h-full flex-col overflow-hidden bg-[radial-gradient(circle_at_top_left,rgba(255,255,255,0.08),transparent_32%),linear-gradient(180deg,rgba(255,255,255,0.02),transparent_22%)]">
      <div className="border-b border-border/60 px-6 py-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-2">
            <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/70 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
              <Sparkles className="h-3.5 w-3.5" />
              GitHub IdeaOS
            </div>
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">Ideas</h1>
              <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
                Capture product and technical ideas as GitHub-versioned markdown files, then evolve them over time.
              </p>
            </div>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="relative min-w-[280px]">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search ideas..."
                className="pl-9"
              />
            </div>
            <Link href="/ideas/new" className={cn(buttonVariants(), "h-8")}>
              <Plus className="h-4 w-4" />
              New Idea
            </Link>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-6">
        {!configured ? (
          <Card className="mx-auto max-w-2xl border-dashed border-border/70 bg-background/80">
            <CardHeader className="space-y-3">
              <div className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Lightbulb className="h-5 w-5" />
              </div>
              <div>
                <CardTitle>Configure GitHub first</CardTitle>
                <CardDescription className="mt-1 max-w-xl">
                  Connect GitHub and configure an Ideas repository in workspace settings before you start capturing ideas.
                </CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <Link href="/settings?tab=ideas" className={buttonVariants({ variant: "outline" })}>
                Open IdeaOS Settings
              </Link>
            </CardContent>
          </Card>
        ) : loading ? (
          <IdeasPageSkeleton />
        ) : filteredIdeas.length === 0 ? (
          <Card className="mx-auto max-w-2xl border-border/70 bg-background/80">
            <CardHeader>
              <CardTitle>{ideas.length === 0 ? "No ideas yet" : "No matching ideas"}</CardTitle>
              <CardDescription>
                {ideas.length === 0
                  ? "Create your first idea and the app will version it directly into your configured GitHub repository."
                  : "Try a different search query or create a new idea."}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Link href="/ideas/new" className={buttonVariants()}>
                <Plus className="h-4 w-4" />
                New Idea
              </Link>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-5">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-sm font-semibold uppercase tracking-[0.14em] text-muted-foreground">Recent</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {filteredIdeas.length} idea{filteredIdeas.length === 1 ? "" : "s"} tracked in GitHub IdeaOS
                </p>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {filteredIdeas.map((idea) => (
                <IdeaCard key={idea.slug} idea={idea} />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
