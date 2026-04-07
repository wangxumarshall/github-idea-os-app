export interface IdeaOSConfig {
  repo_url: string;
  branch: string;
  directory: string;
  token_configured: boolean;
}

export interface IdeaSummary {
  slug: string;
  path: string;
  title: string;
  summary: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface IdeaDocument extends IdeaSummary {
  content: string;
  sha: string;
}
