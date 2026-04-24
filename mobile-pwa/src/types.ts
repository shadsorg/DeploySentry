export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}
