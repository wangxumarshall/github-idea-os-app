"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, FolderGit2, History, Save, Sparkles } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ContentEditor } from "@/features/editor";
import { api } from "@/shared/api";
import type { IdeaDocument } from "@/shared/types";
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

export function IdeaEditorPage() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;
  const [idea, setIdea] = useState<IdeaDocument | null>(null);
  const [draft, setDraft] = useState("");
  const [loading, setLoading] = useState(true);
  const [saveState, setSaveState] = useState<"idle" | "dirty" | "saving" | "saved" | "error">("idle");
  const [saveClock, setSaveClock] = useState(Date.now());
  const [lastSavedAtMs, setLastSavedAtMs] = useState<number | null>(null);

  const draftRef = useRef(draft);
  const syncedContentRef = useRef("");
  const queuedSaveRef = useRef(false);
  const savingRef = useRef(false);

  draftRef.current = draft;

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
    if (!idea) return;
    const nextContent = draftRef.current;
    if (nextContent === syncedContentRef.current) return;
    if (savingRef.current) {
      queuedSaveRef.current = true;
      return;
    }

    savingRef.current = true;
    setSaveState("saving");

    try {
      const updated = await api.updateIdea(idea.slug, {
        title: idea.title,
        content: nextContent,
        tags: idea.tags,
        created_at: idea.created_at,
        sha: idea.sha,
      });

      syncedContentRef.current = nextContent;
      setIdea((current) => (current ? { ...updated, content: current.content } : updated));
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

          <div className="flex items-center justify-between rounded-2xl border border-border/60 bg-background/70 px-4 py-3 text-sm text-muted-foreground">
            <div className="inline-flex items-center gap-2">
              <FolderGit2 className="h-4 w-4" />
              Commit message pattern: <code>update idea: {idea.slug}</code>
            </div>
            <div>Created {idea.created_at}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
