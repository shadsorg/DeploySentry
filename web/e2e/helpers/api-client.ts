const API_BASE = 'http://localhost:8080/api/v1';

interface AuthResponse {
  token: string;
  user: { id: string; email: string; name: string };
}

export class ApiClient {
  private token: string | undefined;

  constructor(token?: string) {
    this.token = token;
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
    };
    const res = await fetch(`${API_BASE}${path}`, { ...init, headers: { ...headers, ...init?.headers } });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`API ${init?.method ?? 'GET'} ${path} failed (${res.status}): ${body}`);
    }
    return res.json() as Promise<T>;
  }

  async register(email: string, password: string, name: string): Promise<string> {
    const res = await this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
    this.token = res.token;
    return res.token;
  }

  async login(email: string, password: string): Promise<string> {
    const res = await this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.token = res.token;
    return res.token;
  }

  async createOrg(name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>('/orgs', {
      method: 'POST',
      body: JSON.stringify({ name, slug }),
    });
  }

  async createProject(orgSlug: string, name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>(`/orgs/${orgSlug}/projects`, {
      method: 'POST',
      body: JSON.stringify({ name, slug }),
    });
  }

  async createApp(orgSlug: string, projectSlug: string, name: string, slug: string) {
    return this.request<{ id: string; name: string; slug: string }>(
      `/orgs/${orgSlug}/projects/${projectSlug}/apps`,
      { method: 'POST', body: JSON.stringify({ name, slug }) },
    );
  }

  async createFlag(projectId: string, data: Record<string, unknown>) {
    return this.request<{ id: string; key: string }>('/flags', {
      method: 'POST',
      body: JSON.stringify({ project_id: projectId, ...data }),
    });
  }

  async createEnvironment(orgSlug: string, name: string, slug: string, isProduction: boolean) {
    return this.request<{ id: string; slug: string }>(`/orgs/${orgSlug}/environments`, {
      method: 'POST',
      body: JSON.stringify({ name, slug, is_production: isProduction }),
    });
  }

  async createWebhook(url: string, events: string[]) {
    return this.request<{ id: string }>('/webhooks', {
      method: 'POST',
      body: JSON.stringify({ url, events, is_active: true }),
    });
  }

  async createApiKey(name: string, scopes: string[]) {
    return this.request<{ id: string; token: string }>('/api-keys', {
      method: 'POST',
      body: JSON.stringify({ name, scopes }),
    });
  }
}
