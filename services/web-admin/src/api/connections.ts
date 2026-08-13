import { api } from './client';
import type { Connection, CreateConnectionPayload, JsonObject, JsonValue, SchemaSnapshot } from '../types';

export interface CreateConnectionResponse {
  connection: Connection;
}

export interface GetConnectionResponse {
  connection: Connection | null;
}

export interface ListConnectionsResponse {
  connections: Connection[];
  meta: { total: number; limit: number; offset: number };
}

export interface DeleteConnectionResponse {
  deleted: boolean;
}

export const connectionsApi = {
  list: async (tenantId: string, workspaceId?: string, limit = 100, offset = 0) => {
    const { data } = await api.get<ListConnectionsResponse>('/v1/connections', {
      params: { tenant_id: tenantId, workspace_id: workspaceId || undefined, limit, offset },
    });
    return data;
  },

  get: async (id: string) => {
    const { data } = await api.get<GetConnectionResponse>(`/v1/connections/${id}`);
    return data;
  },

  create: async (payload: CreateConnectionPayload) => {
    const { data } = await api.post<CreateConnectionResponse>('/v1/connections', payload);
    return data;
  },

  delete: async (id: string) => {
    const { data } = await api.delete<DeleteConnectionResponse>(`/v1/connections/${id}`);
    return data;
  },

  testConnection: async (adapterType: string, params: JsonObject) => {
    const { data } = await api.post<{ ok: boolean; error_message?: string; latency_ms: number }>(
      '/v1/test-connection',
      { adapter_type: adapterType, params }
    );
    return data;
  },

  getSchema: async (connectionId: string) => {
    const { data } = await api.get<{ snapshot?: SchemaSnapshot }>('/v1/schema', {
      params: { connection_id: connectionId },
    });
    return data;
  },

  submitQuery: async (connectionId: string, sql: string) => {
    const { data } = await api.post<{ job_id: string; status: string }>('/v1/query', { connection_id: connectionId, sql });
    return data;
  },

  getJobStatus: async (jobId: string) => {
    const { data } = await api.get<{ job_id: string; status: string; error_message?: string }>('/v1/query/status', {
      params: { job_id: jobId },
    });
    return data;
  },

  getJobResult: async (jobId: string) => {
    const { data } = await api.get<{ job_id: string; rows: Record<string, JsonValue>[]; truncated: boolean }>(
      '/v1/query/result',
      { params: { job_id: jobId, page: 1, page_size: 100 } },
    );
    return data;
  },
};
