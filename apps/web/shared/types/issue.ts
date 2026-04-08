export type IssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type IssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export type IssueAssigneeType = "member" | "agent";

export interface IssueReaction {
  id: string;
  issue_id: string;
  actor_type: string;
  actor_id: string;
  emoji: string;
  created_at: string;
}

export interface Issue {
  id: string;
  workspace_id: string;
  number: number;
  identifier: string;
  title: string;
  description: string | null;
  status: IssueStatus;
  priority: IssuePriority;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
  creator_type: IssueAssigneeType;
  creator_id: string;
  parent_issue_id: string | null;
  repo_url?: string | null;
  idea_id?: string | null;
  idea_slug?: string | null;
  idea_code?: string | null;
  idea_title?: string | null;
  idea_root_issue_id?: string | null;
  idea_root_identifier?: string | null;
  idea_root_title?: string | null;
  is_idea_root?: boolean;
  position: number;
  due_date: string | null;
  reactions?: IssueReaction[];
  created_at: string;
  updated_at: string;
}
