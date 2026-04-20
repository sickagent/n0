import { Title, Button, Paper, TextInput, Select, Stack, Group, Text } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { IconPlus } from '@tabler/icons-react';
import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useAuth } from '../auth/AuthContext';
import { pluginsApi } from '../api/plugins';
import { Modal } from '@mantine/core';

export function Plugins() {
  const { session } = useAuth();
  const tenantId = session?.user_id || '';
  const [opened, { open, close }] = useDisclosure(false);
  const [name, setName] = useState('');
  const [version, setVersion] = useState('1.0.0');
  const [pluginType, setPluginType] = useState('DB_ADAPTER');
  const [endpoint, setEndpoint] = useState('');
  const [protocol, setProtocol] = useState('grpc');

  const mutation = useMutation({
    mutationFn: pluginsApi.register,
    onSuccess: () => {
      notifications.show({ title: 'Success', message: 'Plugin registered', color: 'green' });
      close();
      setName('');
      setVersion('1.0.0');
      setEndpoint('');
    },
    onError: (err: any) => {
      notifications.show({ title: 'Error', message: err?.message || 'Failed to register plugin', color: 'red' });
    },
  });

  const handleSubmit = () => {
    mutation.mutate({
      plugin_type: pluginType,
      name,
      version,
      endpoint,
      protocol,
      tenant_id: tenantId,
      global: false,
    });
  };

  return (
    <div>
      <Group justify="space-between" mb="lg">
        <Title order={2}>Plugins</Title>
        <Button leftSection={<IconPlus size={16} />} onClick={open}>
          Register Plugin
        </Button>
      </Group>

      <Paper withBorder radius="md" shadow="sm" p="md">
        <Text c="dimmed" ta="center" py="xl">
          Plugin registry management coming soon. Use the button above to register external adapters and capabilities.
        </Text>
      </Paper>

      <Modal opened={opened} onClose={close} title="Register Plugin">
        <Stack>
          <TextInput label="Name" value={name} onChange={(e) => setName(e.currentTarget.value)} required />
          <TextInput label="Version" value={version} onChange={(e) => setVersion(e.currentTarget.value)} required />
          <Select
            label="Type"
            data={[
              { value: 'DB_ADAPTER', label: 'Database Adapter' },
              { value: 'AGENT_CAPABILITY', label: 'Agent Capability' },
              { value: 'QUERY_TRANSFORMER', label: 'Query Transformer' },
              { value: 'FORMATTER', label: 'Formatter' },
            ]}
            value={pluginType}
            onChange={(val) => val && setPluginType(val)}
            required
          />
          <TextInput label="Endpoint" value={endpoint} onChange={(e) => setEndpoint(e.currentTarget.value)} required />
          <Select
            label="Protocol"
            data={[
              { value: 'grpc', label: 'gRPC' },
              { value: 'http', label: 'HTTP' },
              { value: 'wasm', label: 'WASM' },
            ]}
            value={protocol}
            onChange={(val) => val && setProtocol(val)}
            required
          />
          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={close}>
              Cancel
            </Button>
            <Button onClick={handleSubmit} loading={mutation.isPending}>
              Register
            </Button>
          </Group>
        </Stack>
      </Modal>
    </div>
  );
}
