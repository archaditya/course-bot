// Typed API client wrapping every endpoint from docs/10-api-contracts.md.
// Components never call fetch() directly — they use these functions or the
// React Query hooks in features/*/hooks.ts.

const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

// ── Auth ──────────────────────────────────────────────────────────────────

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  user: { id: string; email: string };
}

export async function apiSignup(email: string, password: string): Promise<{ id: string; email: string }> {
  return apiFetch('/auth/signup', { method: 'POST', body: { email, password } });
}

export async function apiLogin(email: string, password: string): Promise<AuthTokens> {
  return apiFetch('/auth/login', { method: 'POST', body: { email, password } });
}

export async function apiRefresh(refresh_token: string): Promise<AuthTokens> {
  return apiFetch('/auth/refresh', { method: 'POST', body: { refresh_token } });
}

// ── Projects ──────────────────────────────────────────────────────────────

export interface Project {
  id: string;
  name: string;
  created_at: string;
}

export async function apiCreateProject(name: string): Promise<Project> {
  return apiFetch('/projects', { method: 'POST', body: { name }, auth: true });
}

export async function apiListProjects(cursor?: string): Promise<{ items: Project[]; next_cursor: string }> {
  const q = cursor ? `?cursor=${cursor}&limit=20` : '?limit=20';
  return apiFetch(`/projects${q}`, { auth: true });
}

export async function apiGetProject(id: string): Promise<Project> {
  return apiFetch(`/projects/${id}`, { auth: true });
}

// ── Courses ──────────────────────────────────────────────────────────────

export interface Course {
  id: string;
  title: string;
  status: string;
  created_at: string | number | Date;
}

export async function apiCreateCourse(projectId: string, title: string): Promise<Course> {
  return apiFetch(`/projects/${projectId}/courses`, { method: 'POST', body: { title }, auth: true });
}

export async function apiListCourses(projectId: string, cursor?: string): Promise<{ items: Course[]; next_cursor: string }> {
  const q = cursor ? `?cursor=${cursor}&limit=20` : '?limit=20';
  return apiFetch(`/projects/${projectId}/courses${q}`, { auth: true });
}

export async function apiGetCourse(id: string): Promise<Course> {
  return apiFetch(`/courses/${id}`, { auth: true });
}

export async function apiRenameCourse(id: string, title: string): Promise<Course> {
  return apiFetch(`/courses/${id}`, { method: 'PATCH', body: { title }, auth: true });
}

export async function apiDeleteCourse(id: string): Promise<void> {
  await apiFetch(`/courses/${id}`, { method: 'DELETE', auth: true });
}

// ── Upload ────────────────────────────────────────────────────────────────

export interface UploadResult {
  course_id: string;
  document_ids: string[];
}

export async function apiUpload(
  courseId: string,
  projectId: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<UploadResult> {
  const form = new FormData();
  form.append('file', file);
  form.append('project_id', projectId);

  const token = getToken();
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${BASE}/courses/${courseId}/upload`);
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    });

    xhr.onload = () => {
      if (xhr.status === 202) {
        resolve(JSON.parse(xhr.responseText));
      } else {
        reject(new Error(`Upload failed: ${xhr.status} ${xhr.responseText}`));
      }
    };
    xhr.onerror = () => reject(new Error('Network error during upload'));
    xhr.send(form);
  });
}

// ── Conversations ─────────────────────────────────────────────────────────

export interface Conversation {
  id: string;
  project_id: string;
  title: string;
  created_at: string;
}

export async function apiCreateConversation(projectId: string): Promise<Conversation> {
  return apiFetch('/conversations', { method: 'POST', body: { project_id: projectId }, auth: true });
}

// ── Health ────────────────────────────────────────────────────────────────

export async function apiHealth(): Promise<{ status: string }> {
  return apiFetch('/healthz');
}

// ── Internal helpers ──────────────────────────────────────────────────────

interface FetchOptions {
  method?: string;
  body?: unknown;
  auth?: boolean;
}

async function apiFetch<T>(path: string, opts: FetchOptions = {}): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (opts.auth) {
    const token = getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });

  if (!res.ok) {
    let errMsg = `${res.status}`;
    try {
      const errBody = await res.json();
      errMsg = errBody?.error?.message ?? errMsg;
    } catch {}
    throw new ApiError(errMsg, res.status);
  }

  if (res.status === 204) return undefined as unknown as T;
  return res.json();
}

export class ApiError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message);
    this.name = 'ApiError';
  }
}

export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('access_token');
}

export function setTokens(tokens: { access_token: string; refresh_token: string }): void {
  localStorage.setItem('access_token', tokens.access_token);
  localStorage.setItem('refresh_token', tokens.refresh_token);
}

export function clearTokens(): void {
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
}

// ── Sources ───────────────────────────────────────────────────────────────

export interface AddSourceResult {
  course_id: string;
  document_ids: string[];
}

/**
 * Add a URL-based or text-based source to a course.
 * For file uploads, use apiUpload() instead.
 */
export async function apiAddSource(
  courseId: string,
  sourceType: 'url' | 'text' | 'video_url',
  opts: { url?: string; content?: string; title?: string },
): Promise<AddSourceResult> {
  return apiFetch(`/courses/${courseId}/sources`, {
    method: 'POST',
    body: { source_type: sourceType, ...opts },
    auth: true,
  });
}

// ── Course Status Polling ─────────────────────────────────────────────────

export interface JobStatus {
  id: string;
  stage: string;
  status: string;
  attempts: number;
  last_error?: string;
}

export interface CourseStatus {
  course_id: string;
  status: string;
  jobs: JobStatus[];
}

/**
 * Poll for course processing status. Returns the current course status
 * and all associated jobs. Frontend should poll at 3-5s intervals.
 */
export async function apiGetCourseStatus(courseId: string): Promise<CourseStatus> {
  return apiFetch(`/courses/${courseId}/status`, { auth: true });
}

// ── Chunk Detail (for source panel) ───────────────────────────────────────

export interface ChunkDetail {
  id: string;
  document_id: string;
  content: string;
  title?: string;
  start_timestamp?: number;
  end_timestamp?: number;
  page_number?: number;
}

/**
 * Fetch full chunk details for the source detail panel.
 */
export async function apiGetChunk(chunkId: string): Promise<ChunkDetail> {
  return apiFetch(`/chunks/${chunkId}`, { auth: true });
}
