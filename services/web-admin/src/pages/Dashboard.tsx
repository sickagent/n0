import { Grid, Paper, Text, Group, ThemeIcon, Title, Skeleton } from '@mantine/core';
import { IconDatabase, IconPlug, IconBuilding, IconChartBar } from '@tabler/icons-react';
import { useQuery } from '@tanstack/react-query';
import { useAuth } from '../auth/AuthContext';
import { connectionsApi } from '../api/connections';
import { workspacesApi } from '../api/workspaces';

function StatCard({
  title,
  value,
  icon: Icon,
  loading,
}: {
  title: string;
  value: number | string;
  icon: typeof IconDatabase;
  loading?: boolean;
}) {
  return (
    <Paper withBorder p="md" radius="md" shadow="sm">
      <Group justify="space-between">
        <div>
          <Text c="dimmed" tt="uppercase" fw={700} fz="xs">
            {title}
          </Text>
          {loading ? (
            <Skeleton height={28} width={60} mt={4} />
          ) : (
            <Text fw={700} fz="xl">
              {value}
            </Text>
          )}
        </div>
        <ThemeIcon variant="light" size="xl" radius="md">
          <Icon size="1.5rem" stroke={1.5} />
        </ThemeIcon>
      </Group>
    </Paper>
  );
}

export function Dashboard() {
  const { session } = useAuth();
  const tenantId = session?.user_id || '';

  const { data: connectionsData, isLoading: connectionsLoading } = useQuery({
    queryKey: ['connections', tenantId],
    queryFn: () => connectionsApi.list(tenantId),
    enabled: !!tenantId,
  });

  const { data: workspacesData, isLoading: workspacesLoading } = useQuery({
    queryKey: ['workspaces', tenantId],
    queryFn: () => workspacesApi.list(tenantId),
    enabled: !!tenantId,
  });

  return (
    <div>
      <Title order={2} mb="lg">
        Dashboard
      </Title>
      <Grid>
        <Grid.Col span={{ base: 12, md: 6, lg: 3 }}>
          <StatCard
            title="Connections"
            value={connectionsData?.meta?.total ?? connectionsData?.connections?.length ?? 0}
            icon={IconDatabase}
            loading={connectionsLoading}
          />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6, lg: 3 }}>
          <StatCard title="Plugins" value="-" icon={IconPlug} loading={false} />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6, lg: 3 }}>
          <StatCard
            title="Workspaces"
            value={workspacesData?.meta?.total ?? workspacesData?.workspaces?.length ?? 0}
            icon={IconBuilding}
            loading={workspacesLoading}
          />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6, lg: 3 }}>
          <StatCard title="Queries Today" value="-" icon={IconChartBar} loading={false} />
        </Grid.Col>
      </Grid>

      <Paper withBorder p="md" radius="md" shadow="sm" mt="xl">
        <Title order={4} mb="md">
          Quick Actions
        </Title>
        <Text c="dimmed" size="sm">
          Use the sidebar to manage connections, register plugins, explore schemas, or run queries.
        </Text>
      </Paper>
    </div>
  );
}
