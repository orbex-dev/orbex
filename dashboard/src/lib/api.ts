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
  image: string;
  command?: string[];
  env?: Record<string, string>;
  memory_mb?: number;
  cpu_millicores?: number;
  timeout_seconds?: number;
  schedule?: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

function getApiKey(): string {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('orbex_api_key') || '';
  }
  return '';
}

export function setApiKey(key: string) {
  localStorage.setItem('orbex_api_key', key);
}

export function getStoredApiKey(): string {
  return getApiKey();
}

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${getApiKey()}`,
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

// Jobs
export const api = {
  listJobs: () => apiFetch<Job[]>('/jobs'),
  getJob: (id: string) => apiFetch<Job>(`/jobs/${id}`),
  createJob: (data: CreateJobRequest) =>
    apiFetch<Job>('/jobs', { method: 'POST', body: JSON.stringify(data) }),
  deleteJob: (id: string) =>
    apiFetch<void>(`/jobs/${id}`, { method: 'DELETE' }),

  // Runs
  triggerRun: (jobId: string) =>
    apiFetch<JobRun>(`/jobs/${jobId}/run`, { method: 'POST' }),
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

  // Auth
  register: (email: string, password: string) =>
    apiFetch<{ id: string }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  generateApiKey: (email: string, password: string) =>
    apiFetch<{ key: string }>('/auth/api-keys', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
};
