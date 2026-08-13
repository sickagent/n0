export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
export type JsonObject = Record<string, JsonValue>;

export interface Connection {
  id: string;
  workspace_id: string;
  tenant_id: string;
  name: string;
  adapter_type: string;
  params?: JsonObject;
  created_at: string;
}

export interface CreateConnectionPayload {
  workspace_id: string;
  tenant_id: string;
  name: string;
  adapter_type: string;
  params: JsonObject;
}

export interface Plugin {
  id: string;
  plugin_type: string;
  name: string;
  version: string;
  endpoint: string;
  protocol: string;
  status: string;
}

export interface Workspace {
  id: string;
  tenant_id: string;
  name: string;
  created_at: string;
}

export interface AuthSession {
  user_id: string;
  email: string;
  role: string;
  token: string;
}

export interface LoginPayload {
  email: string;
  password: string;
}

export type RegisterPayload = LoginPayload;

export interface TableInfo {
  name: string;
  columns: ColumnInfo[];
}

export interface ColumnInfo {
  name: string;
  data_type: string;
  nullable: boolean;
}

export interface SchemaSnapshot {
  connection_id: string;
  tables: TableInfo[];
  captured_at: string;
}

export interface ListMeta {
  total: number;
  limit: number;
  offset: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  meta: ListMeta;
}
