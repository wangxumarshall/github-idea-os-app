"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, FilePlus2, Lightbulb } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button, buttonVariants } from "@/components/ui/button";
import { api } from "@/shared/api";
import type { IdeaOSConfig } from "@/shared/types";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

function slugPreview(title: string) {
  return title
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function NewIdeaPage() {
  const router = useRouter();
  const [config, setConfig] = useState<IdeaOSConfig | null>(null);
  const [title, setTitle] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    void api.getIdeaOSConfig().then(setConfig).catch((error) => {
      toast.error(error instanceof Error ? error.message : "Failed to load IdeaOS config");
    });
  }, []);

  const configured = !!config?.repo_url && config?.token_configured;
  const slug = useMemo(() => slugPreview(title), [title]);

  const handleCreate = async () => {
    if (!title.trim() || creating) return;
    setCreating(true);
    try {
      const idea = await api.createIdea({ title: title.trim() });
      router.push(`/ideas/${idea.slug}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create idea");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="flex h-full items-center justify-center px-6 py-8">
      <Card className="w-full max-w-xl border-border/70 bg-background/85 shadow-sm">
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
                Create a GitHub-versioned markdown note under <code>{config?.directory || "ideas"}</code>.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          {!configured ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-muted/30 p-4">
              <div className="flex items-start gap-3">
                <Lightbulb className="mt-0.5 h-4 w-4 text-primary" />
                <div className="space-y-2">
                  <p className="text-sm font-medium">IdeaOS is not configured yet</p>
                  <p className="text-sm text-muted-foreground">
                    Add a repository URL and GitHub token in workspace settings before creating ideas.
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
                <label htmlFor="idea-name" className="text-sm font-medium">
                  Idea name
                </label>
                <Input
                  id="idea-name"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      void handleCreate();
                    }
                  }}
                  placeholder="AI PR Review"
                  autoFocus
                />
              </div>
              <div className="rounded-lg border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                {slug ? (
                  <>
                    File path: <code>{config?.directory || "ideas"}/{slug}.md</code>
                  </>
                ) : (
                  "The file path preview will appear here."
                )}
              </div>
              <div className="flex justify-end">
                <Button onClick={handleCreate} disabled={!title.trim() || creating}>
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
