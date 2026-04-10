"use client";

import Link from "next/link";
import { cn } from "@/lib/utils";
import type { Issue } from "@/shared/types";

interface IssueIdeaBadgeProps {
  issue: Pick<Issue, "idea_code" | "idea_title" | "idea_slug" | "idea_is_system">;
  className?: string;
  compact?: boolean;
}

export function IssueIdeaBadge({ issue, className, compact = false }: IssueIdeaBadgeProps) {
  if (!issue.idea_code && !issue.idea_title && !issue.idea_slug) {
    return null;
  }

  const label = issue.idea_is_system
    ? "Legacy"
    : compact
      ? (issue.idea_code ?? issue.idea_title ?? issue.idea_slug ?? "Idea")
      : [issue.idea_code, issue.idea_title].filter(Boolean).join(" · ");

  const badgeClassName = cn(
    "inline-flex max-w-full items-center rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.16em] text-muted-foreground",
    className,
  );

  if (!issue.idea_slug || issue.idea_is_system) {
    return <span className={badgeClassName}>{label}</span>;
  }

  return (
    <Link href={`/ideas/${issue.idea_slug}`} className={cn(badgeClassName, "hover:border-primary/30 hover:text-foreground")}>
      {label}
    </Link>
  );
}
