import { Title, Paper, Table, Text, Loader, Group, ScrollArea, Badge } from '@mantine/core';
import { IconBuilding } from '@tabler/icons-react';
import { useQuery } from '@tanstack/react-query';
import { workspacesApi } from '../api/workspaces';

const tenantId = 'default';

export function Workspaces() {
  const { data, isLoading } = useQuery({
    queryKey: ['workspaces', tenantId],
    queryFn: () => workspacesApi.list(tenantId),
  });

  const rows = (data?.workspaces || []).map((ws) => (
    <Table.Tr key={ws.id}>
      <Table.Td>
        <Text fw={500}>{ws.name}</Text>
      </Table.Td>
      <Table.Td>
        <Badge leftSection={<IconBuilding size={12} />} variant="light">
          {ws.tenant_id}
        </Badge>
      </Table.Td>
      <Table.Td>
        <Text size="sm" c="dimmed">
          {new Date(ws.created_at).toLocaleString()}
        </Text>
      </Table.Td>
    </Table.Tr>
  ));

  return (
    <div>
      <Title order={2} mb="lg">
        Workspaces
      </Title>
      <Paper withBorder radius="md" shadow="sm" p="md">
        <ScrollArea>
          <Table striped highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Tenant</Table.Th>
                <Table.Th>Created</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {isLoading ? (
                <Table.Tr>
                  <Table.Td colSpan={3}>
                    <Group justify="center" py="xl">
                      <Loader />
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ) : rows.length ? (
                rows
              ) : (
                <Table.Tr>
                  <Table.Td colSpan={3}>
                    <Text ta="center" c="dimmed" py="xl">
                      No workspaces found
                    </Text>
                  </Table.Td>
                </Table.Tr>
              )}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      </Paper>
    </div>
  );
}
