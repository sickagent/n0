import { api } from './client';
import type { Connection, CreateConnectionPayload, TableInfo } from '../types';

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

  testConnection: async (adapterType: string, params: Record<string, any>) => {
    const { data } = await api.post<{ ok: boolean; error_message?: string; latency_ms: number }>(
      '/v1/test-connection',
      { adapter_type: adapterType, params }
    );
    return data;
  },

  getSchema: async (connectionId: string, adapterType: string, params: Record<string, any>) => {
    const { data } = await api.post<{ tables: TableInfo[] }>('/v1/schema', {
      connection_id: connectionId,
      adapter_type: adapterType,
      params,
    });
    return data;
  },

  executeQuery: async (connectionId: string, adapterType: string, params: Record<string, any>, sql: string, limit = 100) => {
    const { data } = await api.post<any>('/v1/execute-query', {
      connection_id: connectionId,
      adapter_type: adapterType,
      params,
      sql,
      limit,
      timeout_seconds: 30,
    });
    return data;
  },
};
