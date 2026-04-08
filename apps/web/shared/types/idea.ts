export interface IdeaOSConfig {
  repo_url: string;
  branch: string;
  directory: string;
  repo_visibility: "private" | "public";
  default_agent_ids: string[];
  github_connected: boolean;
  github_login: string | null;
}

export interface IdeaSummary {
  code: string;
  slug: string;
  path: string;
  title: string;
  summary: string;
  tags: string[];
  project_repo_name: string;
  project_repo_url: string;
  project_repo_status: "creating" | "ready" | "failed";
  provisioning_error?: string;
  root_issue_id?: string;
  created_at: string;
  updated_at: string;
}

export interface IdeaDocument extends IdeaSummary {
  content: string;
  sha: string;
}

export interface IdeaIssuesResponse {
  root_issue: import("./issue").Issue | null;
  child_issues: import("./issue").Issue[];
}

export interface IdeaNameSuggestion {
  name: string;
  slug_suffix: string;
  full_name: string;
}

export interface GitHubAccount {
  id: string;
  login: string;
  avatar_url: string | null;
  profile_url: string | null;
}
