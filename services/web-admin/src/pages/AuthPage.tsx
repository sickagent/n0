import {
  Anchor,
  Button,
  Paper,
  PasswordInput,
  Stack,
  Tabs,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { useState } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

type LocationState = {
  from?: {
    pathname?: string;
  };
};

export function AuthPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isAuthenticated, login, register } = useAuth();
  const [activeTab, setActiveTab] = useState<string | null>('login');

  const [loginEmail, setLoginEmail] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [loginLoading, setLoginLoading] = useState(false);

  const [registerEmail, setRegisterEmail] = useState('');
  const [registerPassword, setRegisterPassword] = useState('');
  const [registerRole, setRegisterRole] = useState('user');
  const [registerLoading, setRegisterLoading] = useState(false);

  if (isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  const redirectTo = (location.state as LocationState | null)?.from?.pathname || '/';

  const handleLogin = async () => {
    setLoginLoading(true);
    try {
      await login({ email: loginEmail, password: loginPassword });
      notifications.show({ title: 'Welcome back', message: 'You are signed in.', color: 'green' });
      navigate(redirectTo, { replace: true });
    } catch (err: any) {
      notifications.show({ title: 'Sign in failed', message: err?.message || 'Unable to sign in.', color: 'red' });
    } finally {
      setLoginLoading(false);
    }
  };

  const handleRegister = async () => {
    setRegisterLoading(true);
    try {
      await register({ email: registerEmail, password: registerPassword, role: registerRole });
      notifications.show({ title: 'Account created', message: 'You are signed in.', color: 'green' });
      navigate('/', { replace: true });
    } catch (err: any) {
      notifications.show({ title: 'Registration failed', message: err?.message || 'Unable to register.', color: 'red' });
    } finally {
      setRegisterLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        padding: '24px',
        background:
          'radial-gradient(circle at top left, rgba(28,126,214,0.18), transparent 28%), linear-gradient(135deg, #f8fbff 0%, #eef4f7 48%, #f7f2ea 100%)',
      }}
    >
      <Paper
        withBorder
        radius="xl"
        shadow="xl"
        p="xl"
        style={{ width: '100%', maxWidth: 460, backdropFilter: 'blur(14px)', background: 'rgba(255,255,255,0.9)' }}
      >
        <Stack gap="lg">
          <div>
            <Text fw={700} c="blue.7" tt="uppercase" size="xs" mb={8}>
              n0 Platform
            </Text>
            <Title order={1} size="h2">
              Sign in to the admin console
            </Title>
            <Text c="dimmed" mt="xs">
              Register a new account or continue with an existing one.
            </Text>
          </div>

          <Tabs value={activeTab} onChange={setActiveTab}>
            <Tabs.List grow>
              <Tabs.Tab value="login">Sign In</Tabs.Tab>
              <Tabs.Tab value="register">Register</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="login" pt="lg">
              <Stack>
                <TextInput
                  label="Email"
                  placeholder="admin@n0.local"
                  value={loginEmail}
                  onChange={(event) => setLoginEmail(event.currentTarget.value)}
                  autoComplete="email"
                />
                <PasswordInput
                  label="Password"
                  placeholder="Your password"
                  value={loginPassword}
                  onChange={(event) => setLoginPassword(event.currentTarget.value)}
                  autoComplete="current-password"
                />
                <Button onClick={handleLogin} loading={loginLoading}>
                  Sign In
                </Button>
                <Text size="sm" c="dimmed">
                  Need an account?{' '}
                  <Anchor component="button" type="button" onClick={() => setActiveTab('register')}>
                    Register here
                  </Anchor>
                </Text>
              </Stack>
            </Tabs.Panel>

            <Tabs.Panel value="register" pt="lg">
              <Stack>
                <TextInput
                  label="Email"
                  placeholder="you@example.com"
                  value={registerEmail}
                  onChange={(event) => setRegisterEmail(event.currentTarget.value)}
                  autoComplete="email"
                />
                <PasswordInput
                  label="Password"
                  placeholder="Create a password"
                  value={registerPassword}
                  onChange={(event) => setRegisterPassword(event.currentTarget.value)}
                  autoComplete="new-password"
                />
                <TextInput
                  label="Role"
                  value={registerRole}
                  onChange={(event) => setRegisterRole(event.currentTarget.value)}
                  description="Defaults to user. Keep this unless you need a custom role."
                />
                <Button onClick={handleRegister} loading={registerLoading}>
                  Create Account
                </Button>
              </Stack>
            </Tabs.Panel>
          </Tabs>
        </Stack>
      </Paper>
    </div>
  );
}
