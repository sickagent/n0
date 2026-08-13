CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    actor_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    success BOOLEAN NOT NULL,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL,
    persisted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_events_tenant_time ON audit_events(tenant_id, occurred_at DESC);
CREATE INDEX idx_audit_events_resource ON audit_events(tenant_id, resource_type, resource_id);

ALTER TABLE plugin_definitions
    ADD COLUMN tenant_id TEXT,
    ADD COLUMN is_global BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN last_heartbeat_at TIMESTAMPTZ,
    ADD COLUMN last_error TEXT,
    ADD COLUMN consecutive_failures INT NOT NULL DEFAULT 0,
    ADD CONSTRAINT plugin_status_check CHECK (status IN ('registered', 'validated', 'active', 'degraded', 'disabled'));

CREATE INDEX idx_plugin_definitions_health ON plugin_definitions(status, updated_at);
CREATE INDEX idx_plugin_definitions_tenant ON plugin_definitions(tenant_id);

-- Every persisted entity uses a UUID primary key. Keep the former natural
-- tenant/plugin key as a uniqueness constraint for idempotent enablement.
ALTER TABLE tenant_plugins DROP CONSTRAINT tenant_plugins_pkey;
ALTER TABLE tenant_plugins ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE tenant_plugins ADD CONSTRAINT tenant_plugins_pkey PRIMARY KEY (id);
ALTER TABLE tenant_plugins ADD CONSTRAINT tenant_plugins_tenant_plugin_key UNIQUE (tenant_id, plugin_id);

CREATE TABLE plugin_healthchecks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES plugin_definitions(id) ON DELETE CASCADE,
    healthy BOOLEAN NOT NULL,
    latency_ms BIGINT NOT NULL,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_plugin_healthchecks_plugin_time ON plugin_healthchecks(plugin_id, checked_at DESC);
