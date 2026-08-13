DROP TABLE IF EXISTS plugin_healthchecks;
ALTER TABLE tenant_plugins DROP CONSTRAINT IF EXISTS tenant_plugins_tenant_plugin_key;
ALTER TABLE tenant_plugins DROP CONSTRAINT IF EXISTS tenant_plugins_pkey;
ALTER TABLE tenant_plugins ADD CONSTRAINT tenant_plugins_pkey PRIMARY KEY (tenant_id, plugin_id);
ALTER TABLE tenant_plugins DROP COLUMN IF EXISTS id;
DROP INDEX IF EXISTS idx_plugin_definitions_tenant;
DROP INDEX IF EXISTS idx_plugin_definitions_health;
ALTER TABLE plugin_definitions
    DROP CONSTRAINT IF EXISTS plugin_status_check,
    DROP COLUMN IF EXISTS consecutive_failures,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS last_heartbeat_at,
    DROP COLUMN IF EXISTS is_global,
    DROP COLUMN IF EXISTS tenant_id;
DROP TABLE IF EXISTS audit_events;
