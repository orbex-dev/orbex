export type SourceType = 'image' | 'script' | 'upload' | 'dockerfile' | 'github' | 'compose';

export interface Job {
  id: string;
  user_id: string;
  name: string;
  image: string;
  command?: string[];
  env?: Record<string, string>;
  memory_mb: number;
  cpu_millicores: number;
  timeout_seconds: number;
  schedule?: string;
  webhook_token?: string;
  script?: string;
  script_lang?: string;
  source_type: SourceType;
  github_repo?: string;
  github_branch?: string;
  dockerfile_path?: string;
  source_config?: Record<string, unknown>;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface JobRun {
  id: string;
  job_id: string;
  user_id: string;
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'paused' | 'cancelled';
  container_id?: string;
  exit_code?: number;
  error_message?: string;
  started_at?: string;
  finished_at?: string;
  paused_at?: string;
  heartbeat_at?: string;
  duration_ms?: number;
  logs_tail?: string;
  created_at: string;
}

export interface CreateJobRequest {
  name: string;
  image?: string;
  command?: string[];
  env?: Record<string, string>;
  memory_mb?: number;
  cpu_millicores?: number;
  timeout_seconds?: number;
  schedule?: string;
  script?: string;
  script_lang?: string;
  source_type?: SourceType;
  github_repo?: string;
  github_branch?: string;
  github_token_id?: string;
  dockerfile_path?: string;
  source_config?: Record<string, unknown>;
}

export interface UpdateJobRequest {
  name?: string;
  image?: string;
  command?: string[];
  env?: Record<string, string>;
  memory_mb?: number;
  cpu_millicores?: number;
  timeout_seconds?: number;
  schedule?: string;
  is_active?: boolean;
  script?: string;
  script_lang?: string;
  source_type?: SourceType;
  github_repo?: string;
  github_branch?: string;
  dockerfile_path?: string;
  source_config?: Record<string, unknown>;
}

export interface TriggerRunRequest {
  timeout_seconds?: number;
  env?: Record<string, string>;
  command?: string[];
}

export interface User {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include', // Send cookies with every request
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.message || body.error || `API error ${res.status}`);
  }

  if (res.status === 204) return {} as T;
  return res.json();
}

// Auth (session-based)
export const auth = {
  login: (email: string, password: string) =>
    apiFetch<User>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  logout: () =>
    apiFetch<{ status: string }>('/auth/logout', { method: 'POST' }),
  getMe: () =>
    apiFetch<User>('/auth/me'),
  register: (email: string, password: string) =>
    apiFetch<User>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  changePassword: (currentPassword: string, newPassword: string) =>
    apiFetch<{ status: string }>('/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),
};

// Jobs
export const api = {
  listJobs: () => apiFetch<Job[]>('/jobs'),
  getJob: (id: string) => apiFetch<Job>(`/jobs/${id}`),
  createJob: (data: CreateJobRequest) =>
    apiFetch<Job>('/jobs', { method: 'POST', body: JSON.stringify(data) }),
  updateJob: (id: string, data: UpdateJobRequest) =>
    apiFetch<Job>(`/jobs/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteJob: (id: string) =>
    apiFetch<void>(`/jobs/${id}`, { method: 'DELETE' }),

  // Runs
  triggerRun: (jobId: string) =>
    apiFetch<JobRun>(`/jobs/${jobId}/run`, { method: 'POST' }),
  triggerRunWithOverrides: (jobId: string, overrides: TriggerRunRequest) =>
    apiFetch<JobRun>(`/jobs/${jobId}/run`, { method: 'POST', body: JSON.stringify(overrides) }),
  listRuns: (jobId: string) =>
    apiFetch<JobRun[]>(`/jobs/${jobId}/runs`),
  getRun: (runId: string) =>
    apiFetch<JobRun>(`/runs/${runId}`),
  pauseRun: (runId: string) =>
    apiFetch<{ status: string }>(`/runs/${runId}/pause`, { method: 'POST' }),
  resumeRun: (runId: string) =>
    apiFetch<{ status: string }>(`/runs/${runId}/resume`, { method: 'POST' }),
  killRun: (runId: string) =>
    apiFetch<{ status: string }>(`/runs/${runId}/kill`, { method: 'POST' }),
  getRunLogs: (runId: string) =>
    apiFetch<{ logs: string }>(`/runs/${runId}/logs`),

  // Webhooks
  generateWebhook: (jobId: string) =>
    apiFetch<{ webhook_token: string; trigger_url: string }>(`/jobs/${jobId}/webhook`, { method: 'POST' }),

  // API Keys (for settings page — still needed for programmatic access)
  generateApiKey: (email: string, password: string) =>
    apiFetch<{ key: string }>('/auth/api-keys', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
};

// GitHub integration
export const github = {
  getStatus: () =>
    apiFetch<{ connected: boolean; github_username?: string; connected_at?: string }>('/github/status'),
  listRepos: () =>
    apiFetch<Array<{ full_name: string; name: string; owner: string; default_branch: string; private: boolean; description: string; html_url: string }>>('/github/repos'),
  listBranches: (owner: string, repo: string) =>
    apiFetch<string[]>(`/github/repos/${owner}/${repo}/branches`),
};

// File uploads
export const uploads = {
  upload: async (jobId: string, files: FileList | File[]) => {
    const formData = new FormData();
    Array.from(files).forEach(f => formData.append('files', f));
    const res = await fetch(`${API_BASE}/jobs/${jobId}/upload`, {
      method: 'POST',
      credentials: 'include',
      body: formData,
    });
    if (!res.ok) throw new Error('Upload failed');
    return res.json();
  },
  listFiles: (jobId: string) =>
    apiFetch<Array<{ filename: string; size: number; last_modified: string }>>(`/jobs/${jobId}/files`),
  deleteFile: (jobId: string, filename: string) =>
    apiFetch<void>(`/jobs/${jobId}/files/${filename}`, { method: 'DELETE' }),
};

// Client-side search helper
export function searchJobs(jobs: Job[], query: string): Job[] {
  if (!query.trim()) return jobs;
  const q = query.toLowerCase();
  return jobs.filter(j =>
    j.name.toLowerCase().includes(q) ||
    j.image.toLowerCase().includes(q) ||
    j.id.includes(q)
  );
}
