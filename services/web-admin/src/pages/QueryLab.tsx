import { Title, Paper, Select, Textarea, Button, Group, Stack, Text, Table, ScrollArea, JsonInput } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconPlayerPlay } from '@tabler/icons-react';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useAuth } from '../auth/useAuth';
import { connectionsApi } from '../api/connections';
import type { JsonValue } from '../types';
import { errorMessage } from '../utils/errors';

type QueryResult = {
  job_id: string;
  rows: Record<string, JsonValue>[];
  truncated: boolean;
};

const pollDelayMs = 500;
const maxPollAttempts = 120;

export function QueryLab() {
  const { session } = useAuth();
  const tenantId = session?.user_id || '';
  const { data: connectionsData } = useQuery({
    queryKey: ['connections', tenantId],
    queryFn: () => connectionsApi.list(tenantId),
    enabled: !!tenantId,
  });

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [sql, setSql] = useState('SELECT 1');
  const [result, setResult] = useState<QueryResult | null>(null);
  const [loading, setLoading] = useState(false);

  const connections = connectionsData?.connections || [];
  const selectedConn = connections.find((c) => c.id === selectedId);

  const handleRun = async () => {
    if (!selectedConn) {
      notifications.show({ title: 'Error', message: 'Select a connection first', color: 'red' });
      return;
    }
    setLoading(true);
    try {
      const submitted = await connectionsApi.submitQuery(selectedConn.id, sql);
      for (let attempt = 0; attempt < maxPollAttempts; attempt += 1) {
        const status = await connectionsApi.getJobStatus(submitted.job_id);
        if (status.status === 'failed') {
          throw new Error(status.error_message || 'Query failed');
        }
        if (status.status === 'success') {
          setResult(await connectionsApi.getJobResult(submitted.job_id));
          return;
        }
        await new Promise((resolve) => window.setTimeout(resolve, pollDelayMs));
      }
      throw new Error('Query timed out while waiting for a result');
    } catch (err: unknown) {
      notifications.show({ title: 'Error', message: errorMessage(err, 'Query failed'), color: 'red' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <Title order={2} mb="lg">
        Query Lab
      </Title>

      <Paper withBorder radius="md" shadow="sm" p="md">
        <Stack>
          <Group align="flex-end">
            <Select
              label="Connection"
              placeholder="Pick a connection"
              data={connections.map((c) => ({ value: c.id, label: `${c.name} (${c.adapter_type})` }))}
              value={selectedId}
              onChange={setSelectedId}
              style={{ minWidth: 300 }}
            />
            <Button
              leftSection={<IconPlayerPlay size={16} />}
              onClick={handleRun}
              loading={loading}
              disabled={!selectedId}
            >
              Run Query
            </Button>
          </Group>

          <Textarea
            label="SQL"
            value={sql}
            onChange={(e) => setSql(e.currentTarget.value)}
            minRows={4}
            style={{ fontFamily: 'monospace' }}
          />
        </Stack>
      </Paper>

      {result && (
        <Paper withBorder radius="md" shadow="sm" p="md" mt="md">
          <Text fw={700} mb="sm">
            Result
          </Text>
          {result.rows.length > 0 ? (
            <ScrollArea>
              <Table striped>
                <Table.Thead>
                  <Table.Tr>
                    {Object.keys(result.rows[0]).map((col) => (
                      <Table.Th key={col}>{col}</Table.Th>
                    ))}
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {result.rows.map((row, idx) => (
                    <Table.Tr key={idx}>
                      {Object.keys(result.rows[0]).map((col) => (
                        <Table.Td key={col}>
                          <Text size="sm">
                            {typeof row[col] === 'string' ? row[col] : JSON.stringify(row[col])}
                          </Text>
                        </Table.Td>
                      ))}
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </ScrollArea>
          ) : (
            <JsonInput value={JSON.stringify(result, null, 2)} readOnly autosize minRows={4} />
          )}
        </Paper>
      )}
    </div>
  );
}
