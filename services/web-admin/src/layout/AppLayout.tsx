import {
  AppShell,
  Burger,
  Group,
  NavLink,
  ScrollArea,
  Text,
  useMantineColorScheme,
  ActionIcon,
  Box,
  Avatar,
  Menu,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import {
  IconLayoutDashboard,
  IconPlug,
  IconDatabase,
  IconBuilding,
  IconTerminal2,
  IconSun,
  IconMoon,
  IconUser,
  IconLogout,
} from '@tabler/icons-react';
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

const navItems = [
  { label: 'Dashboard', path: '/', icon: IconLayoutDashboard },
  { label: 'Connections', path: '/connections', icon: IconDatabase },
  { label: 'Plugins', path: '/plugins', icon: IconPlug },
  { label: 'Workspaces', path: '/workspaces', icon: IconBuilding },
  { label: 'Query Lab', path: '/query-lab', icon: IconTerminal2 },
];

export function AppLayout() {
  const [mobileOpened, { toggle: toggleMobile }] = useDisclosure();
  const [desktopOpened, { toggle: toggleDesktop }] = useDisclosure(true);
  const { colorScheme, toggleColorScheme } = useMantineColorScheme();
  const location = useLocation();
  const navigate = useNavigate();
  const { session, logout } = useAuth();

  const links = navItems.map((item) => (
    <NavLink
      key={item.path}
      component={Link}
      to={item.path}
      label={item.label}
      leftSection={<item.icon size="1.2rem" stroke={1.5} />}
      active={location.pathname === item.path || location.pathname.startsWith(`${item.path}/`)}
      variant="filled"
      onClick={() => mobileOpened && toggleMobile()}
    />
  ));

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{
        width: 260,
        breakpoint: 'sm',
        collapsed: { mobile: !mobileOpened, desktop: !desktopOpened },
      }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Group>
            <Burger opened={mobileOpened} onClick={toggleMobile} hiddenFrom="sm" size="sm" />
            <Burger opened={desktopOpened} onClick={toggleDesktop} visibleFrom="sm" size="sm" />
            <Text fw={700} size="lg">
              n0 Admin
            </Text>
          </Group>

          <Group>
            <ActionIcon
              variant="default"
              onClick={() => toggleColorScheme()}
              size={30}
              aria-label="Toggle color scheme"
            >
              {colorScheme === 'dark' ? <IconSun size={16} /> : <IconMoon size={16} />}
            </ActionIcon>

            <Menu shadow="md" width={200}>
              <Menu.Target>
                <ActionIcon variant="default" size={30}>
                  <Avatar size={24} radius="xl">
                    <IconUser size={16} />
                  </Avatar>
                </ActionIcon>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Label>{session?.email || 'Signed in'}</Menu.Label>
                <Menu.Item
                  leftSection={<IconLogout size={14} />}
                  onClick={() => {
                    logout();
                    navigate('/auth', { replace: true });
                  }}
                >
                  Logout
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar>
        <AppShell.Section grow component={ScrollArea}>
          <Box py="sm">{links}</Box>
        </AppShell.Section>
        <AppShell.Section p="md">
          <Text size="xs" c="dimmed">
            n0 Platform v0.1.0
          </Text>
        </AppShell.Section>
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  );
}
