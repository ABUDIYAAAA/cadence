export interface Competition {
  id: string;
  event_id: string;
  slug: string;
  name: string;
  description?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CompetitionListResponse {
  competitions: Competition[];
  total: number;
}

export interface CreateCompetitionRequest {
  slug: string;
  name: string;
  description?: string;
}

export interface UpdateCompetitionRequest {
  slug?: string;
  name?: string;
  description?: string;
}
