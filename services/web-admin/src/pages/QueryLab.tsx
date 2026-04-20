import { Title, Paper, Select, Textarea, Button, Group, Stack, Text, Table, ScrollArea, JsonInput } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconPlayerPlay } from '@tabler/icons-react';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useAuth } from '../auth/AuthContext';
import { connectionsApi } from '../api/connections';

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
  const [result, setResult] = useState<any>(null);
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
      const res = await connectionsApi.executeQuery(
        selectedConn.id,
        selectedConn.adapter_type,
        selectedConn.params,
        sql,
        100
      );
      setResult(res);
    } catch (err: any) {
      notifications.show({ title: 'Error', message: err?.message || 'Query failed', color: 'red' });
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
          {result.error_message ? (
            <Text c="red">{result.error_message}</Text>
          ) : result.rows && result.rows.length > 0 ? (
            <ScrollArea>
              <Table striped>
                <Table.Thead>
                  <Table.Tr>
                    {result.columns.map((col: string) => (
                      <Table.Th key={col}>{col}</Table.Th>
                    ))}
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {result.rows.map((row: any, idx: number) => (
                    <Table.Tr key={idx}>
                      {result.columns.map((col: string) => (
                        <Table.Td key={col}>
                          <Text size="sm">
                            {row.values?.[result.columns.indexOf(col)] ?? JSON.stringify(row[col])}
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
