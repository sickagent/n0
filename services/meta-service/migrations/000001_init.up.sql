CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX idx_workspaces_tenant_name ON workspaces(tenant_id, name);

CREATE TABLE connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    adapter_type TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_connections_workspace ON connections(workspace_id);
CREATE INDEX idx_connections_tenant ON connections(tenant_id);

CREATE TABLE schema_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    tables JSONB NOT NULL DEFAULT '[]',
    captured_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_schema_snapshots_connection ON schema_snapshots(connection_id);

CREATE TABLE plugin_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_type TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    author TEXT,
    endpoint TEXT,
    protocol TEXT NOT NULL DEFAULT 'grpc',
    status TEXT NOT NULL DEFAULT 'registered',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX idx_plugin_definitions_name_version ON plugin_definitions(name, version);

CREATE TABLE plugin_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES plugin_definitions(id) ON DELETE CASCADE,
    capability_name TEXT NOT NULL,
    capability_schema JSONB
);

CREATE INDEX idx_plugin_capabilities_plugin ON plugin_capabilities(plugin_id);

CREATE TABLE tenant_plugins (
    tenant_id TEXT NOT NULL,
    plugin_id UUID NOT NULL REFERENCES plugin_definitions(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (tenant_id, plugin_id)
);
