import type { Issue, IssueStatus, IssuePriority, IssueAssigneeType } from "./issue";
import type { MemberRole } from "./workspace";

export interface UpdateIdeaOSConfigRequest {
  repo_url?: string;
  branch?: string;
  directory?: string;
  repo_visibility?: "private" | "public";
  default_agent_ids?: string[];
}

export interface CreateIdeaRequest {
  raw_input: string;
  selected_name: string;
}

export interface RecommendIdeaNameRequest {
  raw_input: string;
}

export interface RecommendIdeaNameResponse {
  next_code: string;
  suggestions: import("./idea").IdeaNameSuggestion[];
}

export interface UpdateIdeaRequest {
  title: string;
  content: string;
  base_title: string;
  base_content: string;
  tags?: string[];
  base_tags?: string[];
  created_at: string;
  base_created_at: string;
  sha: string;
}

// Issue API
export interface CreateIssuePayload {
  title: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType;
  assignee_id?: string;
  parent_issue_id?: string;
  repo_url?: string;
  due_date?: string;
  attachment_ids?: string[];
}

export interface UpdateIssueRequest {
  title?: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  position?: number;
  repo_url?: string | null;
  due_date?: string | null;
}

export interface ListIssuesParams {
  limit?: number;
  offset?: number;
  workspace_id?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_id?: string;
}

export interface ListIssuesResponse {
  issues: Issue[];
  total: number;
}

export interface UpdateMeRequest {
  name?: string;
  avatar_url?: string;
}

export interface CreateMemberRequest {
  email: string;
  role?: MemberRole;
}

export interface UpdateMemberRequest {
  role: MemberRole;
}

// Personal Access Tokens
export interface PersonalAccessToken {
  id: string;
  name: string;
  token_prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface CreatePersonalAccessTokenRequest {
  name: string;
  expires_in_days?: number;
}

export interface CreatePersonalAccessTokenResponse extends PersonalAccessToken {
  token: string;
}

// Pagination
export interface PaginationParams {
  limit?: number;
  offset?: number;
}
