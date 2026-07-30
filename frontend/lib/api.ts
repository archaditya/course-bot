// One client owns auth headers, error envelopes and token refresh. UI code uses
// typed resource functions instead of scattered fetch/XHR implementations.
export const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export interface User {
	id: string;
	full_name: string;
	email: string;
	role?: 'user' | 'admin';
	is_disabled?: boolean;
}
export interface AuthTokens { access_token: string; refresh_token: string; user: User }
export class ApiError extends Error { constructor(message: string, public readonly status: number, public readonly code?: string) { super(message); this.name = 'ApiError'; } }

export async function apiSignup(fullName: string, email: string, password: string): Promise<User> { return request('/auth/signup', { method: 'POST', body: { full_name: fullName, email, password } }); }
export async function apiLogin(email: string, password: string): Promise<AuthTokens> { return request('/auth/login', { method: 'POST', body: { email, password } }); }
export async function apiMe(): Promise<User> { return request('/auth/me', { auth: true }); }
export async function apiRefresh(refresh_token: string): Promise<Pick<AuthTokens, 'access_token' | 'refresh_token'>> { return request('/auth/refresh', { method: 'POST', body: { refresh_token } }); }

// ── Admin Telemetry & Management ──────────────────────────────────────────────
export interface SystemStats {
	total_users: number;
	active_users: number;
	restricted_users: number;
	total_conversations: number;
	total_documents: number;
	total_messages: number;
	total_chunks: number;
}

export interface UserUsageStat {
	id: string;
	email: string;
	full_name: string;
	role: 'user' | 'admin';
	is_disabled: boolean;
	created_at: string;
	conversation_count: number;
	document_count: number;
	message_count: number;
	chunk_count: number;
}

export async function apiGetAdminStats(): Promise<SystemStats> { return request('/admin/stats', { auth: true }); }
export async function apiGetAdminUsers(): Promise<{ items: UserUsageStat[] }> { return request('/admin/users', { auth: true }); }
export async function apiUpdateUserStatus(userId: string, isDisabled: boolean): Promise<void> { return request(`/admin/users/${userId}/status`, { method: 'PATCH', body: { is_disabled: isDisabled }, auth: true }); }
export async function apiUpdateUserRole(userId: string, role: 'user' | 'admin'): Promise<void> { return request(`/admin/users/${userId}/role`, { method: 'PATCH', body: { role }, auth: true }); }

// ── Conversations ───────────────────────────────────────────────────────────
// Every conversation belongs directly to the signed-in user (no project or
// course wrapper). Each conversation owns its own knowledge base — the
// Documents added to it are only ever visible/searchable within it.
export interface Conversation { id: string; title: string; created_at: string; updated_at: string }
export interface ConversationMessage {
	id: string;
	role: 'user' | 'assistant';
	content: string;
	citations?: Array<{ chunk_id: string; document_id: string; start_timestamp?: number; title?: string }>;
}
export async function apiCreateConversation(): Promise<Conversation> { return request('/conversations', { method: 'POST', auth: true }); }
export async function apiListConversations(): Promise<{ items: Conversation[] }> { return request('/conversations', { auth: true }); }
export async function apiDeleteConversation(id: string): Promise<void> { return request(`/conversations/${id}`, { method: 'DELETE', auth: true }); }
export async function apiUpdateConversationTitle(id: string, title: string): Promise<void> { return request(`/conversations/${id}`, { method: 'PATCH', body: { title }, auth: true }); }
export async function apiGetConversationMessages(conversationId: string): Promise<{ items: ConversationMessage[] }> {
	return request(`/conversations/${conversationId}/messages`, { auth: true });
}

// ── Documents (sources) ─────────────────────────────────────────────────────
// One Document per source — a file, a URL, pasted text, or one file out of a
// ZIP. Each tracks its own indexing status independently; there is no
// collection-level "course" status anymore.
export type DocumentStatus = 'UPLOADING' | 'UPLOADED' | 'PARSING' | 'NORMALIZING' | 'CHUNKING' | 'EMBEDDING' | 'INDEXED' | 'FAILED';
export interface DocumentItem {
	id: string;
	original_filename: string;
	source_type: string;
	source_url?: string;
	status: DocumentStatus;
	created_at: string;
	summary?: string;
	ai_summary?: string;
	ai_questions?: string[];
	ai_overview?: string;
}
export interface UploadResult { conversation_id: string; document_ids: string[] }

export async function apiListDocuments(conversationId: string): Promise<{ items: DocumentItem[] }> {
	return request(`/conversations/${conversationId}/documents`, { auth: true });
}
export async function apiDeleteDocument(conversationId: string, documentId: string): Promise<void> {
	return request(`/conversations/${conversationId}/documents/${documentId}`, { method: 'DELETE', auth: true });
}

// Uploads a single file (pdf/docx/txt/md/srt/vtt) or a .zip archive — the
// backend auto-detects .zip and fans it out into one Document per supported
// file inside it.
export function apiUploadDocument(conversationId: string, file: File, onProgress?: (pct: number) => void): Promise<UploadResult> {
	const token = getToken();
	const form = new FormData();
	form.append('file', file);
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open('POST', `${BASE}/conversations/${conversationId}/documents`);
		if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
		xhr.upload.onprogress = (event) => { if (event.lengthComputable) onProgress?.(Math.round(event.loaded / event.total * 100)); };
		xhr.onload = () => { if (xhr.status === 202) resolve(JSON.parse(xhr.responseText)); else reject(toApiError(xhr.status, xhr.responseText)); };
		xhr.onerror = () => reject(new ApiError('Network error during upload', 0));
		xhr.send(form);
	});
}

// Adds a YouTube/web URL or pasted text as a source (the other two tabs of
// the "Add Document" modal — file/zip uploads use apiUploadDocument above).
export async function apiAddSource(
	conversationId: string,
	sourceType: 'url' | 'text' | 'video_url',
	options: { url?: string; content?: string; title?: string }
): Promise<UploadResult> {
	return request(`/conversations/${conversationId}/documents/source`, { method: 'POST', body: { source_type: sourceType, ...options }, auth: true });
}

export interface JobStatus { id: string; stage: string; status: string; attempts: number; last_error?: string }
export interface DocumentStatusDetail { document_id: string; status: DocumentStatus; jobs: JobStatus[] }
export async function apiGetDocumentStatus(documentId: string): Promise<DocumentStatusDetail> {
	return request(`/documents/${documentId}/status`, { auth: true });
}

// ── Chunks (citations) ──────────────────────────────────────────────────────
export interface ChunkDetail {
	id: string;
	document_id: string;
	content: string;
	title?: string;
	start_timestamp?: number;
	end_timestamp?: number;
	page_number?: number;
	document_name?: string;
	source_type?: string;
	source_url?: string;
}
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
