// One client owns auth headers, error envelopes and token refresh. UI code uses
// typed resource functions instead of scattered fetch/XHR implementations.
export const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export interface User { id: string; full_name: string; email: string }
export interface AuthTokens { access_token: string; refresh_token: string; user: User }
export class ApiError extends Error { constructor(message: string, public readonly status: number, public readonly code?: string) { super(message); this.name = 'ApiError'; } }

export async function apiSignup(fullName: string, email: string, password: string): Promise<User> { return request('/auth/signup', { method: 'POST', body: { full_name: fullName, email, password } }); }
export async function apiLogin(email: string, password: string): Promise<AuthTokens> { return request('/auth/login', { method: 'POST', body: { email, password } }); }
export async function apiMe(): Promise<User> { return request('/auth/me', { auth: true }); }
export async function apiRefresh(refresh_token: string): Promise<Pick<AuthTokens, 'access_token' | 'refresh_token'>> { return request('/auth/refresh', { method: 'POST', body: { refresh_token } }); }

export interface Project { id: string; name: string; created_at: string }
export async function apiCreateProject(name: string): Promise<Project> { return request('/projects', { method: 'POST', body: { name }, auth: true }); }
export async function apiListProjects(cursor?: string): Promise<{ items: Project[]; next_cursor?: string }> { return request(`/projects?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { auth: true }); }
export async function apiGetProject(id: string): Promise<Project> { return request(`/projects/${id}`, { auth: true }); }

export interface Collection { id: string; title: string; status: string; created_at: string }
// Course is kept only as a TypeScript compatibility alias during the API rename.
export type Course = Collection;
export async function apiCreateCollection(projectId: string, title: string): Promise<Collection> { return request(`/projects/${projectId}/collections`, { method: 'POST', body: { title }, auth: true }); }
export async function apiListCollections(projectId: string, cursor?: string): Promise<{ items: Collection[]; next_cursor?: string }> { return request(`/projects/${projectId}/collections?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { auth: true }); }
export async function apiGetCollection(id: string): Promise<Collection> { return request(`/collections/${id}`, { auth: true }); }
export async function apiRenameCollection(id: string, title: string): Promise<Collection> { return request(`/collections/${id}`, { method: 'PATCH', body: { title }, auth: true }); }
export async function apiDeleteCollection(id: string): Promise<void> { return request(`/collections/${id}`, { method: 'DELETE', auth: true }); }
export const apiCreateCourse = apiCreateCollection;
export const apiListCourses = apiListCollections;
export const apiGetCourse = apiGetCollection;
export const apiRenameCourse = apiRenameCollection;
export const apiDeleteCourse = apiDeleteCollection;

export interface UploadResult { course_id: string; document_ids: string[] }
export function apiUpload(collectionId: string, projectId: string, file: File, onProgress?: (pct: number) => void): Promise<UploadResult> {
	const token = getToken(); const form = new FormData(); form.append('file', file); form.append('project_id', projectId);
	return new Promise((resolve, reject) => { const xhr = new XMLHttpRequest(); xhr.open('POST', `${BASE}/collections/${collectionId}/upload`); if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`); xhr.upload.onprogress = (event) => { if (event.lengthComputable) onProgress?.(Math.round(event.loaded / event.total * 100)); }; xhr.onload = () => { if (xhr.status === 202) resolve(JSON.parse(xhr.responseText)); else reject(toApiError(xhr.status, xhr.responseText)); }; xhr.onerror = () => reject(new ApiError('Network error during upload', 0)); xhr.send(form); });
}
export interface AddSourceResult extends UploadResult {}
export async function apiAddSource(collectionId: string, sourceType: 'url' | 'text' | 'video_url', options: { url?: string; content?: string; title?: string }): Promise<AddSourceResult> { return request(`/collections/${collectionId}/sources`, { method: 'POST', body: { source_type: sourceType, ...options }, auth: true }); }
export interface JobStatus { id: string; stage: string; status: string; attempts: number; last_error?: string }
export interface CourseStatus { course_id: string; status: string; jobs: JobStatus[] }
export async function apiGetCourseStatus(id: string): Promise<CourseStatus> { return request(`/collections/${id}/status`, { auth: true }); }

export interface Conversation { id: string; project_id: string; title: string; created_at: string }
export async function apiCreateConversation(projectId: string): Promise<Conversation> { return request('/conversations', { method: 'POST', body: { project_id: projectId }, auth: true }); }
export interface ChunkDetail { id: string; document_id: string; content: string; title?: string; start_timestamp?: number; end_timestamp?: number; page_number?: number }
export async function apiGetChunk(chunkId: string): Promise<ChunkDetail> { return request(`/chunks/${chunkId}`, { auth: true }); }
export async function apiHealth(): Promise<{ status: string }> { return request('/healthz'); }

type Options = { method?: string; body?: unknown; auth?: boolean; retry?: boolean };
let refreshInFlight: Promise<void> | null = null;
async function refreshSession() { if (!refreshInFlight) refreshInFlight = (async () => { const refresh = typeof window === 'undefined' ? null : localStorage.getItem('refresh_token'); if (!refresh) throw new ApiError('Your session has expired.', 401); const tokens = await apiRefresh(refresh); setTokens(tokens); })().finally(() => { refreshInFlight = null; }); return refreshInFlight; }
async function request<T>(path: string, options: Options = {}): Promise<T> {
	const headers: Record<string, string> = { 'Content-Type': 'application/json', Accept: 'application/json' }; const token = options.auth ? getToken() : null; if (token) headers.Authorization = `Bearer ${token}`;
	const response = await fetch(`${BASE}${path}`, { method: options.method ?? 'GET', headers, body: options.body === undefined ? undefined : JSON.stringify(options.body) });
	if (response.status === 401 && options.auth && options.retry !== false) { try { await refreshSession(); return request(path, { ...options, retry: false }); } catch { clearTokens(); } }
	if (!response.ok) { const payload = await response.text(); throw toApiError(response.status, payload); }
	if (response.status === 204) return undefined as T; return response.json() as Promise<T>;
}
function toApiError(status: number, payload: string) { try { const data = JSON.parse(payload); return new ApiError(data?.error?.message ?? `Request failed (${status})`, status, data?.error?.code); } catch { return new ApiError(`Request failed (${status})`, status); } }
export function getToken(): string | null { return typeof window === 'undefined' ? null : localStorage.getItem('access_token'); }
export function setTokens(tokens: Pick<AuthTokens, 'access_token' | 'refresh_token'>): void { localStorage.setItem('access_token', tokens.access_token); localStorage.setItem('refresh_token', tokens.refresh_token); }
export function clearTokens(): void { localStorage.removeItem('access_token'); localStorage.removeItem('refresh_token'); }
export async function apiListConversations(projectId: string): Promise<{ items: Conversation[] }> {
	return request(`/projects/${projectId}/conversations`, { auth: true });
}