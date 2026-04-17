import { api } from './client';
import type { Workspace } from '../types';

export interface ListWorkspacesResponse {
  workspaces: Workspace[];
  meta: { total: number; limit: number; offset: number };
}

export const workspacesApi = {
  list: async (tenantId: string, limit = 100, offset = 0) => {
    const { data } = await api.get<ListWorkspacesResponse>('/v1/workspaces', {
      params: { tenant_id: tenantId, limit, offset },
    });
    return data;
  },
};
