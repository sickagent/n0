import {
  Title,
  Button,
  Paper,
  Table,
  Group,
  Badge,
  ActionIcon,
  Modal,
  TextInput,
  Select,
  Stack,
  JsonInput,
  Text,
  Loader,
  ScrollArea,
  Accordion,
  Code,
  Tooltip,
  Flex,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { IconTrash, IconEye, IconRefresh, IconPlug, IconPlus } from '@tabler/icons-react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useAuth } from '../auth/AuthContext';
import { connectionsApi } from '../api/connections';
import { workspacesApi } from '../api/workspaces';
import type { Connection, TableInfo } from '../types';

const adapterOptions = [
  { value: 'postgres', label: 'PostgreSQL' },
  { value: 'clickhouse', label: 'ClickHouse' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'sqlite', label: 'SQLite' },
  { value: 'mssql', label: 'MS SQL Server' },
  { value: 'bigquery', label: 'BigQuery' },
];

const defaultParamsByAdapter: Record<string, Record<string, any>> = {
  postgres: { host: 'postgres', port: '5432', user: 'postgres', password: 'postgres', database: 'meta', sslmode: 'disable' },
  clickhouse: { host: 'clickhouse', port: '9000', user: 'default', password: '', database: 'default' },
  mysql: { host: 'mysql', port: '3306', user: 'root', password: 'root', database: 'meta' },
  sqlite: { path: ':memory:' },
  mssql: { host: 'localhost', port: '1433', user: 'sa', password: '', database: 'meta' },
  bigquery: { project_id: '', location: 'US' },
};

export function Connections() {
  const { session } = useAuth();
  const tenantId = session?.user_id || '';
  const queryClient = useQueryClient();
  const [createOpened, { open: openCreate, close: closeCreate }] = useDisclosure(false);
  const [schemaOpened, { open: openSchema, close: closeSchema }] = useDisclosure(false);
  const [activeConnection, setActiveConnection] = useState<Connection | null>(null);
  const [schemaLoading, setSchemaLoading] = useState(false);
  const [schemaTables, setSchemaTables] = useState<TableInfo[]>([]);

  const [newName, setNewName] = useState('');
  const [newAdapter, setNewAdapter] = useState('postgres');
  const [newParams, setNewParams] = useState(JSON.stringify(defaultParamsByAdapter['postgres'], null, 2));

  const { data: workspacesData } = useQuery({
    queryKey: ['workspaces', tenantId],
    queryFn: () => workspacesApi.list(tenantId),
    enabled: !!tenantId,
  });

  const workspaces = workspacesData?.workspaces || [];
  const defaultWorkspace = workspaces.find((w) => w.name === 'Default Workspace') || workspaces[0];
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string>(defaultWorkspace?.id || '');

  useEffect(() => {
    if (defaultWorkspace && !selectedWorkspaceId) {
      setSelectedWorkspaceId(defaultWorkspace.id);
    }
  }, [defaultWorkspace, selectedWorkspaceId]);

  const workspaceId = selectedWorkspaceId || defaultWorkspace?.id || '';

  const { data, isLoading } = useQuery({
    queryKey: ['connections', tenantId, workspaceId],
    queryFn: () => connectionsApi.list(tenantId, workspaceId),
    enabled: !!workspaceId,
  });

  const createMutation = useMutation({
    mutationFn: connectionsApi.create,
    onSuccess: () => {
      notifications.show({ title: 'Success', message: 'Connection created', color: 'green' });
      queryClient.invalidateQueries({ queryKey: ['connections', tenantId] });
      closeCreate();
      resetForm();
    },
    onError: (err: any) => {
      notifications.show({ title: 'Error', message: err?.message || 'Failed to create connection', color: 'red' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: connectionsApi.delete,
    onSuccess: () => {
      notifications.show({ title: 'Success', message: 'Connection deleted', color: 'green' });
      queryClient.invalidateQueries({ queryKey: ['connections', tenantId] });
    },
    onError: (err: any) => {
      notifications.show({ title: 'Error', message: err?.message || 'Failed to delete connection', color: 'red' });
    },
  });

  const resetForm = () => {
    setNewName('');
    setNewAdapter('postgres');
    setNewParams(JSON.stringify(defaultParamsByAdapter['postgres'], null, 2));
  };

  const handleCreate = () => {
    let params: Record<string, any> = {};
    try {
      params = JSON.parse(newParams);
    } catch {
      notifications.show({ title: 'Error', message: 'Invalid JSON in params', color: 'red' });
      return;
    }
    if (!workspaceId) {
      notifications.show({ title: 'Error', message: 'No workspace available. Create a workspace first.', color: 'red' });
      return;
    }
    createMutation.mutate({
      workspace_id: workspaceId,
      tenant_id: tenantId,
      name: newName,
      adapter_type: newAdapter,
      params,
    });
  };

  const handleTest = async () => {
    let params: Record<string, any> = {};
    try {
      params = JSON.parse(newParams);
    } catch {
      notifications.show({ title: 'Error', message: 'Invalid JSON in params', color: 'red' });
      return;
    }
    try {
      const res = await connectionsApi.testConnection(newAdapter, params);
      if (res.ok) {
        notifications.show({ title: 'Connected', message: `Latency: ${res.latency_ms}ms`, color: 'green' });
      } else {
        notifications.show({ title: 'Failed', message: res.error_message || 'Connection failed', color: 'red' });
      }
    } catch (err: any) {
      notifications.show({ title: 'Error', message: err?.message || 'Test failed', color: 'red' });
    }
  };

  const handleViewSchema = async (conn: Connection) => {
    setActiveConnection(conn);
    openSchema();
    setSchemaLoading(true);
    try {
      const res = await connectionsApi.getSchema(conn.id, conn.adapter_type, conn.params);
      setSchemaTables(res.tables || []);
    } catch (err: any) {
      notifications.show({ title: 'Error', message: err?.message || 'Failed to load schema', color: 'red' });
    } finally {
      setSchemaLoading(false);
    }
  };

  const rows = (data?.connections || []).map((conn) => (
    <Table.Tr key={conn.id}>
      <Table.Td>
        <Text fw={500}>{conn.name}</Text>
      </Table.Td>
      <Table.Td>
        <Badge leftSection={<IconPlug size={12} />} variant="light">
          {conn.adapter_type}
        </Badge>
      </Table.Td>
      <Table.Td>
        <Text size="sm" c="dimmed">
          {conn.workspace_id}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" c="dimmed">
          {new Date(conn.created_at).toLocaleString()}
        </Text>
      </Table.Td>
      <Table.Td>
        <Group gap="xs">
          <Tooltip label="View schema">
            <ActionIcon variant="subtle" color="blue" onClick={() => handleViewSchema(conn)}>
              <IconEye size={16} />
            </ActionIcon>
          </Tooltip>
          <Tooltip label="Delete">
            <ActionIcon
              variant="subtle"
              color="red"
              onClick={() => deleteMutation.mutate(conn.id)}
              loading={deleteMutation.variables === conn.id && deleteMutation.isPending}
            >
              <IconTrash size={16} />
            </ActionIcon>
          </Tooltip>
        </Group>
      </Table.Td>
    </Table.Tr>
  ));

  return (
    <div>
      <Flex justify="space-between" align="center" mb="lg">
        <Title order={2}>Connections</Title>
        <Group>
          <Select
            placeholder="Select workspace"
            data={workspaces.map((w) => ({ value: w.id, label: w.name }))}
            value={workspaceId}
            onChange={(val) => val && setSelectedWorkspaceId(val)}
            disabled={workspaces.length === 0}
            style={{ minWidth: 200 }}
          />
          <Button leftSection={<IconPlus size={16} />} onClick={openCreate}>
            Add Connection
          </Button>
        </Group>
      </Flex>

      <Paper withBorder radius="md" shadow="sm" p="md">
        <ScrollArea>
          <Table striped highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Adapter</Table.Th>
                <Table.Th>Workspace</Table.Th>
                <Table.Th>Created</Table.Th>
                <Table.Th>Actions</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {isLoading ? (
                <Table.Tr>
                  <Table.Td colSpan={5}>
                    <Group justify="center" py="xl">
                      <Loader />
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ) : rows.length ? (
                rows
              ) : (
                <Table.Tr>
                  <Table.Td colSpan={5}>
                    <Text ta="center" c="dimmed" py="xl">
                      No connections found
                    </Text>
                  </Table.Td>
                </Table.Tr>
              )}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      </Paper>

      <Modal opened={createOpened} onClose={closeCreate} title="Create Connection" size="lg">
        <Stack>
          <TextInput label="Name" value={newName} onChange={(e) => setNewName(e.currentTarget.value)} required />
          <Select
            label="Adapter"
            data={adapterOptions}
            value={newAdapter}
            onChange={(val) => {
              if (val) {
                setNewAdapter(val);
                setNewParams(JSON.stringify(defaultParamsByAdapter[val], null, 2));
              }
            }}
            required
          />
          <JsonInput
            label="Parameters (JSON)"
            value={newParams}
            onChange={setNewParams}
            formatOnBlur
            autosize
            minRows={6}
          />
          <Text size="xs" c="dimmed">
            Tip: inside Docker use service names as hosts: <Code>postgres</Code>, <Code>clickhouse</Code>, <Code>mysql</Code>. Using <Code>localhost</Code> will fail.
          </Text>
          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={closeCreate}>
              Cancel
            </Button>
            <Button variant="light" leftSection={<IconRefresh size={16} />} onClick={handleTest}>
              Test Connection
            </Button>
            <Button onClick={handleCreate} loading={createMutation.isPending}>
              Create
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={schemaOpened} onClose={closeSchema} title={`Schema: ${activeConnection?.name}`} size="xl">
        {schemaLoading ? (
          <Group justify="center" py="xl">
            <Loader />
          </Group>
        ) : schemaTables.length === 0 ? (
          <Text ta="center" c="dimmed" py="xl">
            No tables found
          </Text>
        ) : (
          <ScrollArea h={500}>
            <Accordion>
              {schemaTables.map((table) => (
                <Accordion.Item key={table.name} value={table.name}>
                  <Accordion.Control>
                    <Group>
                      <IconEye size={16} />
                      <Text fw={500}>{table.name}</Text>
                      <Badge size="sm" variant="light">
                        {table.columns.length} columns
                      </Badge>
                    </Group>
                  </Accordion.Control>
                  <Accordion.Panel>
                    <Table>
                      <Table.Thead>
                        <Table.Tr>
                          <Table.Th>Column</Table.Th>
                          <Table.Th>Type</Table.Th>
                          <Table.Th>Nullable</Table.Th>
                        </Table.Tr>
                      </Table.Thead>
                      <Table.Tbody>
                        {table.columns.map((col) => (
                          <Table.Tr key={col.name}>
                            <Table.Td>{col.name}</Table.Td>
                            <Table.Td>
                              <Code>{col.data_type}</Code>
                            </Table.Td>
                            <Table.Td>{col.nullable ? 'Yes' : 'No'}</Table.Td>
                          </Table.Tr>
                        ))}
                      </Table.Tbody>
                    </Table>
                  </Accordion.Panel>
                </Accordion.Item>
              ))}
            </Accordion>
          </ScrollArea>
        )}
      </Modal>
    </div>
  );
}
