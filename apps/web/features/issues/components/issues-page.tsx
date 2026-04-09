"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useStore } from "zustand";
import { toast } from "sonner";
import { ChevronRight, ListTodo } from "lucide-react";
import type { IssueStatus } from "@/shared/types";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/features/auth";
import { useIssueStore } from "@/features/issues/store";
import { useIssueViewStore, initFilterWorkspaceSync, registerViewStoreForWorkspaceSync } from "@/features/issues/stores/view-store";
import { useIssuesScopeStore } from "@/features/issues/stores/issues-scope-store";
import { ViewStoreProvider } from "@/features/issues/stores/view-store-context";
import { filterIssues } from "@/features/issues/utils/filter";
import { BOARD_STATUSES } from "@/features/issues/config";
import { useWorkspaceStore } from "@/features/workspace";
import { WorkspaceAvatar } from "@/features/workspace";
import { api } from "@/shared/api";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { IssuesHeader } from "./issues-header";
import { BoardView } from "./board-view";
import { ListView } from "./list-view";
import { BatchActionToolbar } from "./batch-action-toolbar";
import { myIssuesViewStore } from "@/features/my-issues/stores/my-issues-view-store";
import { MyIssuesHeader } from "@/features/my-issues/components/my-issues-header";

export function IssuesPage() {
  const [pageMode, setPageMode] = useState<"my" | "workspace">("my");
  const user = useAuthStore((s) => s.user);
  const allIssues = useIssueStore((s) => s.issues);
  const loading = useIssueStore((s) => s.loading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const scope = useIssuesScopeStore((s) => s.scope);
  const viewMode = useIssueViewStore((s) => s.viewMode);
  const statusFilters = useIssueViewStore((s) => s.statusFilters);
  const priorityFilters = useIssueViewStore((s) => s.priorityFilters);
  const assigneeFilters = useIssueViewStore((s) => s.assigneeFilters);
  const includeNoAssignee = useIssueViewStore((s) => s.includeNoAssignee);
  const creatorFilters = useIssueViewStore((s) => s.creatorFilters);
  const myViewMode = useStore(myIssuesViewStore, (s) => s.viewMode);
  const myStatusFilters = useStore(myIssuesViewStore, (s) => s.statusFilters);
  const myPriorityFilters = useStore(myIssuesViewStore, (s) => s.priorityFilters);
  const myScope = useStore(myIssuesViewStore, (s) => s.scope);

  useEffect(() => {
    initFilterWorkspaceSync();
    registerViewStoreForWorkspaceSync(myIssuesViewStore);
  }, []);

  useEffect(() => {
    useIssueSelectionStore.getState().clear();
  }, [pageMode, viewMode, scope, myViewMode, myScope]);

  // Scope pre-filter: narrow by assignee type
  const workspaceScopedIssues = useMemo(() => {
    if (scope === "members")
      return allIssues.filter((i) => i.assignee_type === "member");
    if (scope === "agents")
      return allIssues.filter((i) => i.assignee_type === "agent");
    return allIssues;
  }, [allIssues, scope]);

  const workspaceIssues = useMemo(
    () => filterIssues(workspaceScopedIssues, { statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters }),
    [workspaceScopedIssues, statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters],
  );

  const workspaceVisibleStatuses = useMemo(() => {
    if (statusFilters.length > 0)
      return BOARD_STATUSES.filter((s) => statusFilters.includes(s));
    return BOARD_STATUSES;
  }, [statusFilters]);

  const workspaceHiddenStatuses = useMemo(
    () => BOARD_STATUSES.filter((s) => !workspaceVisibleStatuses.includes(s)),
    [workspaceVisibleStatuses],
  );

  const myAgentIds = useMemo(() => {
    if (!user) return new Set<string>();
    return new Set(
      agents.filter((agent) => agent.owner_id === user.id).map((agent) => agent.id),
    );
  }, [agents, user]);

  const assignedToMe = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (issue) => issue.assignee_type === "member" && issue.assignee_id === user.id,
    );
  }, [allIssues, user]);

  const myAgentIssues = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (issue) => issue.assignee_type === "agent" && issue.assignee_id && myAgentIds.has(issue.assignee_id),
    );
  }, [allIssues, user, myAgentIds]);

  const createdByMe = useMemo(() => {
    if (!user) return [];
    return allIssues.filter(
      (issue) => issue.creator_type === "member" && issue.creator_id === user.id,
    );
  }, [allIssues, user]);

  const myScopedIssues = useMemo(() => {
    switch (myScope) {
      case "assigned":
        return assignedToMe;
      case "agents":
        return myAgentIssues;
      case "created":
        return createdByMe;
      default:
        return assignedToMe;
    }
  }, [assignedToMe, createdByMe, myAgentIssues, myScope]);

  const myIssues = useMemo(
    () =>
      filterIssues(myScopedIssues, {
        statusFilters: myStatusFilters,
        priorityFilters: myPriorityFilters,
        assigneeFilters: [],
        includeNoAssignee: false,
        creatorFilters: [],
      }),
    [myPriorityFilters, myScopedIssues, myStatusFilters],
  );

  const myVisibleStatuses = useMemo(() => {
    if (myStatusFilters.length > 0) {
      return BOARD_STATUSES.filter((status) => myStatusFilters.includes(status));
    }
    return BOARD_STATUSES;
  }, [myStatusFilters]);

  const myHiddenStatuses = useMemo(
    () => BOARD_STATUSES.filter((status) => !myVisibleStatuses.includes(status)),
    [myVisibleStatuses],
  );

  const handleMoveIssue = useCallback(
    (issueId: string, newStatus: IssueStatus, newPosition?: number) => {
      // Auto-switch to manual sort so drag ordering is preserved
      const viewState = pageMode === "my" ? myIssuesViewStore.getState() : useIssueViewStore.getState();
      if (viewState.sortBy !== "position") {
        viewState.setSortBy("position");
        viewState.setSortDirection("asc");
      }

      const updates: Partial<{ status: IssueStatus; position: number }> = {
        status: newStatus,
      };
      if (newPosition !== undefined) updates.position = newPosition;

      useIssueStore.getState().updateIssue(issueId, updates);

      api.updateIssue(issueId, updates).catch(() => {
        toast.error("Failed to move issue");
        api.listIssues({ limit: 200 }).then((res) => {
          useIssueStore.getState().setIssues(res.issues);
        }).catch(console.error);
      });
    },
    [pageMode]
  );

  if (loading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <Skeleton className="h-5 w-5 rounded" />
          <Skeleton className="h-4 w-32" />
        </div>
        <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
          <Skeleton className="h-5 w-24" />
          <Skeleton className="h-8 w-24" />
        </div>
        <div className="flex flex-1 min-h-0 gap-4 overflow-x-auto p-4">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex min-w-52 flex-1 flex-col gap-2">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-24 w-full rounded-lg" />
              <Skeleton className="h-24 w-full rounded-lg" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      {/* Header 1: Workspace breadcrumb */}
      <div className="flex h-12 shrink-0 items-center gap-1.5 border-b px-4">
        <WorkspaceAvatar name={workspace?.name ?? "W"} size="sm" />
        <span className="text-sm text-muted-foreground">
          {workspace?.name ?? "Workspace"}
        </span>
        <ChevronRight className="h-3 w-3 text-muted-foreground" />
        <span className="text-sm font-medium">Issues</span>
      </div>

      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex items-center gap-1">
          <Button
            variant={pageMode === "my" ? "default" : "outline"}
            size="sm"
            onClick={() => setPageMode("my")}
          >
            My Issues
          </Button>
          <Button
            variant={pageMode === "workspace" ? "default" : "outline"}
            size="sm"
            onClick={() => setPageMode("workspace")}
          >
            Workspace
          </Button>
        </div>
      </div>

      {pageMode === "my" ? (
        <>
          <MyIssuesHeader allIssues={myScopedIssues} />
          <ViewStoreProvider store={myIssuesViewStore}>
            {myScopedIssues.length === 0 ? (
              <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 text-muted-foreground">
                <ListTodo className="h-10 w-10 text-muted-foreground/40" />
                <p className="text-sm">No issues assigned to you</p>
                <p className="text-xs">Issues you create or are assigned to will appear here.</p>
              </div>
            ) : (
              <div className="flex flex-col flex-1 min-h-0">
                {myViewMode === "board" ? (
                  <BoardView
                    issues={myIssues}
                    allIssues={myScopedIssues}
                    visibleStatuses={myVisibleStatuses}
                    hiddenStatuses={myHiddenStatuses}
                    onMoveIssue={handleMoveIssue}
                  />
                ) : (
                  <ListView issues={myIssues} visibleStatuses={myVisibleStatuses} />
                )}
              </div>
            )}
            {myViewMode === "list" && <BatchActionToolbar />}
          </ViewStoreProvider>
        </>
      ) : (
        <>
          <IssuesHeader scopedIssues={workspaceScopedIssues} />
          <ViewStoreProvider store={useIssueViewStore}>
            {workspaceScopedIssues.length === 0 ? (
              <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 text-muted-foreground">
                <ListTodo className="h-10 w-10 text-muted-foreground/40" />
                <p className="text-sm">No issues yet</p>
                <p className="text-xs">Create an issue to get started.</p>
              </div>
            ) : (
              <div className="flex flex-col flex-1 min-h-0">
                {viewMode === "board" ? (
                  <BoardView
                    issues={workspaceIssues}
                    allIssues={workspaceScopedIssues}
                    visibleStatuses={workspaceVisibleStatuses}
                    hiddenStatuses={workspaceHiddenStatuses}
                    onMoveIssue={handleMoveIssue}
                  />
                ) : (
                  <ListView issues={workspaceIssues} visibleStatuses={workspaceVisibleStatuses} />
                )}
              </div>
            )}
            {viewMode === "list" && <BatchActionToolbar />}
          </ViewStoreProvider>
        </>
      )}
    </div>
  );
}
