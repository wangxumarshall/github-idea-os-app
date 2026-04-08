"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, ArrowLeft, Bug, FolderGit2, History, Plus, RotateCcw, Save, Shield, Sparkles } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ContentEditor } from "@/features/editor";
import { useIssueStore } from "@/features/issues";
import { useModalStore } from "@/features/modals";
import { api } from "@/shared/api";
import type { IdeaDocument, Issue } from "@/shared/types";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

function relativeSavedLabel(savedAtMs: number | null) {
  if (!savedAtMs) return "Not saved yet";
  const deltaSeconds = Math.max(0, Math.floor((Date.now() - savedAtMs) / 1000));
  if (deltaSeconds < 5) return "Saved just now";
  if (deltaSeconds < 60) return `Last saved ${deltaSeconds}s ago`;
  const deltaMinutes = Math.floor(deltaSeconds / 60);
  if (deltaMinutes < 60) return `Last saved ${deltaMinutes}m ago`;
  return "Saved earlier";
}

function compareIssueDates(left: Issue, right: Issue) {
  return new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime();
}

function IdeaIssueLink({ issue }: { issue: Issue }) {
  return (
    <Link href={`/issues/${issue.id}`} className="flex items-center justify-between gap-3 rounded-xl border border-border/60 bg-background/70 px-3 py-3 transition-colors hover:bg-accent/30">
      <div className="min-w-0">
        <div className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">{issue.identifier}</div>
        <div className="truncate text-sm font-medium">{issue.title}</div>
        <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
          <span>{issue.status.replaceAll("_", " ")}</span>
          {issue.repo_url && <span>{issue.repo_url.replace(/^https?:\/\/github\.com\//, "")}</span>}
        </div>
      </div>
      <Badge variant="outline" className="shrink-0 rounded-full px-2.5 py-0.5 text-[11px]">
        {issue.priority}
      </Badge>
    </Link>
  );
}

export function IdeaEditorPage() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;
  const [idea, setIdea] = useState<IdeaDocument | null>(null);
  const [draft, setDraft] = useState("");
  const [loading, setLoading] = useState(true);
  const [saveState, setSaveState] = useState<"idle" | "dirty" | "saving" | "saved" | "error">("idle");
  const [saveClock, setSaveClock] = useState(Date.now());
  const [lastSavedAtMs, setLastSavedAtMs] = useState<number | null>(null);
  const [retryingRepo, setRetryingRepo] = useState(false);
  const issues = useIssueStore((s) => s.issues);

  const draftRef = useRef(draft);
  const ideaRef = useRef<IdeaDocument | null>(idea);
  const syncedIdeaRef = useRef<IdeaDocument | null>(null);
  const syncedContentRef = useRef("");
  const queuedSaveRef = useRef(false);
  const savingRef = useRef(false);

  draftRef.current = draft;
  ideaRef.current = idea;

  useEffect(() => {
    const interval = window.setInterval(() => setSaveClock(Date.now()), 1000);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      try {
        const nextIdea = await api.getIdea(slug);
        if (cancelled) return;
        ideaRef.current = nextIdea;
        syncedIdeaRef.current = nextIdea;
        setIdea(nextIdea);
        setDraft(nextIdea.content);
        syncedContentRef.current = nextIdea.content;
        setSaveState("saved");
        setLastSavedAtMs(Date.now());
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : "Failed to load idea");
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
  }, [slug]);

  const saveDraft = async () => {
    const currentIdea = ideaRef.current;
    const syncedIdea = syncedIdeaRef.current;
    if (!currentIdea || !syncedIdea) return;
    const nextContent = draftRef.current;
    if (nextContent === syncedContentRef.current) return;
    if (savingRef.current) {
      queuedSaveRef.current = true;
      return;
    }

    savingRef.current = true;
    setSaveState("saving");

    try {
      const updated = await api.updateIdea(currentIdea.slug, {
        title: currentIdea.title,
        content: nextContent,
        base_title: syncedIdea.title,
        base_content: syncedIdea.content,
        tags: currentIdea.tags,
        base_tags: syncedIdea.tags,
        created_at: currentIdea.created_at,
        base_created_at: syncedIdea.created_at,
        sha: syncedIdea.sha,
      });

      const persistedIdea = { ...updated, content: nextContent };
      syncedContentRef.current = nextContent;
      syncedIdeaRef.current = persistedIdea;
      ideaRef.current = persistedIdea;
      setIdea(persistedIdea);
      setLastSavedAtMs(Date.now());
      if (draftRef.current === nextContent) {
        setSaveState("saved");
      } else {
        setSaveState("dirty");
        queuedSaveRef.current = true;
      }
    } catch (error) {
      setSaveState("error");
      toast.error(error instanceof Error ? error.message : "Failed to save idea");
    } finally {
      savingRef.current = false;
      if (queuedSaveRef.current) {
        queuedSaveRef.current = false;
        void saveDraft();
      }
    }
  };

  useEffect(() => {
    if (!idea) return;
    if (draft === syncedContentRef.current) return;

    setSaveState("dirty");
    const timeout = window.setTimeout(() => {
      void saveDraft();
    }, 3000);

    return () => {
      window.clearTimeout(timeout);
    };
  }, [draft, idea]);

  const saveLabel = useMemo(() => {
    void saveClock;
    if (!idea) return "Loading...";
    switch (saveState) {
      case "saving":
        return "Saving...";
      case "dirty":
        return "Unsaved changes";
      case "error":
        return "Save failed";
      default:
        return relativeSavedLabel(lastSavedAtMs);
    }
  }, [idea, lastSavedAtMs, saveClock, saveState]);

  const rootIssue = useMemo(() => {
    if (!idea?.root_issue_id) return null;
    return issues.find((issue) => issue.id === idea.root_issue_id) ?? null;
  }, [idea?.root_issue_id, issues]);

  const childIssues = useMemo(() => {
    if (!idea) return [];
    return issues
      .filter((issue) => issue.idea_slug === idea.slug && issue.id !== idea.root_issue_id)
      .sort(compareIssueDates);
  }, [idea, issues]);

  const handleRetryRepo = async () => {
    if (!idea || retryingRepo) return;
    setRetryingRepo(true);
    try {
      await api.retryIdeaRepo(idea.slug);
      setIdea((current) =>
        current
          ? { ...current, project_repo_status: "creating", provisioning_error: "" }
          : current
      );
      toast.success("Repository provisioning retried");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to retry repository provisioning");
    } finally {
      setRetryingRepo(false);
    }
  };

  const openIdeaIssueModal = (kind: "bug" | "feature" | "stability") => {
    if (!idea) return;
    useModalStore.getState().open("create-issue", {
      idea_slug: idea.slug,
      parent_issue_id: idea.root_issue_id,
      repo_url: idea.project_repo_url,
      status: "todo",
      kind,
    });
  };

  if (loading || !idea) {
    return (
      <div className="space-y-4 px-6 py-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col overflow-hidden bg-[linear-gradient(180deg,rgba(255,255,255,0.03),transparent_18%)]">
      <div className="border-b border-border/60 px-6 py-4">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
          <div className="space-y-3">
            <Link href="/ideas" className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "-ml-2 w-fit")}>
              <ArrowLeft className="h-4 w-4" />
              Back to Ideas
            </Link>
            <div className="space-y-2">
              <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/70 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                Idea Editor
              </div>
              <div>
                <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">{idea.code}</div>
                <h1 className="text-3xl font-semibold tracking-tight">{idea.title}</h1>
                <p className="mt-1 text-sm text-muted-foreground">
                  Stored as <code>{idea.path}</code> and committed to GitHub on autosave.
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3 py-1.5">
              <Save className="h-3.5 w-3.5" />
              {saveLabel}
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3 py-1.5">
              <History className="h-3.5 w-3.5" />
              Updated {idea.updated_at}
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3 py-1.5">
              <FolderGit2 className="h-3.5 w-3.5" />
              Repo {idea.project_repo_status}
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-6">
        <div className="mx-auto flex max-w-5xl flex-col gap-4">
          <Card className="border-border/70 bg-background/85">
            <CardHeader className="gap-3 border-b border-border/60">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <CardTitle className="text-base">Markdown notes</CardTitle>
                  <CardDescription className="mt-1">
                    Type freely. The app will update frontmatter and commit changes to GitHub after 3 seconds of inactivity.
                  </CardDescription>
                </div>
                <div className="flex flex-wrap gap-2">
                  {idea.tags.map((tag) => (
                    <Badge key={tag} variant="secondary" className="rounded-full px-2.5 py-0.5">
                      {tag}
                    </Badge>
                  ))}
                  {idea.tags.length === 0 && (
                    <Badge variant="outline" className="rounded-full px-2.5 py-0.5">
                      No tags yet
                    </Badge>
                  )}
                  <Badge variant={idea.project_repo_status === "ready" ? "secondary" : idea.project_repo_status === "failed" ? "destructive" : "outline"} className="rounded-full px-2.5 py-0.5">
                    {idea.project_repo_status}
                  </Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              <div className="min-h-[560px] px-5 py-5">
                <ContentEditor
                  key={idea.slug}
                  defaultValue={idea.content}
                  className="min-h-[520px]"
                  debounceMs={200}
                  onUpdate={(markdown) => setDraft(markdown)}
                  placeholder="Write the idea here..."
                />
              </div>
            </CardContent>
          </Card>

          <div className="space-y-3 rounded-2xl border border-border/60 bg-background/70 px-4 py-3 text-sm text-muted-foreground">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="inline-flex items-center gap-2">
                <FolderGit2 className="h-4 w-4" />
                Commit message pattern: <code>update idea: {idea.slug}</code>
              </div>
              <div>Created {idea.created_at}</div>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <a href={idea.project_repo_url} target="_blank" rel="noreferrer" className="text-primary hover:underline">
                {idea.project_repo_name}
              </a>
              {idea.project_repo_status === "failed" && (
                <>
                  <span className="inline-flex items-center gap-1 text-destructive">
                    <AlertCircle className="h-4 w-4" />
                    {idea.provisioning_error || "Repository provisioning failed"}
                  </span>
                  <Button size="sm" variant="outline" onClick={handleRetryRepo} disabled={retryingRepo}>
                    <RotateCcw className="h-3.5 w-3.5" />
                    {retryingRepo ? "Retrying..." : "Retry repo creation"}
                  </Button>
                </>
              )}
            </div>
          </div>

          <Card className="border-border/70 bg-background/85">
            <CardHeader className="gap-3 border-b border-border/60">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <CardTitle className="text-base">Delivery issues</CardTitle>
                  <CardDescription className="mt-1">
                    Use a root issue to drive implementation, then open child issues for bugs, features, and stability work.
                  </CardDescription>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" variant="outline" onClick={() => openIdeaIssueModal("bug")} disabled={!idea.root_issue_id}>
                    <Bug className="h-3.5 w-3.5" />
                    Bug issue
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => openIdeaIssueModal("feature")} disabled={!idea.root_issue_id}>
                    <Plus className="h-3.5 w-3.5" />
                    Feature issue
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => openIdeaIssueModal("stability")} disabled={!idea.root_issue_id}>
                    <Shield className="h-3.5 w-3.5" />
                    Stability issue
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-5 p-5">
              <div className="space-y-2">
                <div className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">Root issue</div>
                {rootIssue ? (
                  <IdeaIssueLink issue={rootIssue} />
                ) : (
                  <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-3 py-4 text-sm text-muted-foreground">
                    This idea does not have a root issue yet.
                  </div>
                )}
              </div>

              <div className="space-y-2">
                <div className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">Child issues</div>
                {childIssues.length > 0 ? (
                  <div className="space-y-2">
                    {childIssues.map((issue) => (
                      <IdeaIssueLink key={issue.id} issue={issue} />
                    ))}
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-3 py-4 text-sm text-muted-foreground">
                    No child issues yet. Create one when this idea needs a bug fix, feature increment, or stability workstream.
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
