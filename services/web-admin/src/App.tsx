import { Navigate, Route, Routes } from 'react-router-dom';
import { RequireAuth } from './auth/RequireAuth';
import { AppLayout } from './layout/AppLayout';
import { AuthPage } from './pages/AuthPage';
import { Dashboard } from './pages/Dashboard';
import { Connections } from './pages/Connections';
import { Plugins } from './pages/Plugins';
import { Workspaces } from './pages/Workspaces';
import { QueryLab } from './pages/QueryLab';

function App() {
  return (
    <Routes>
      <Route path="/auth" element={<AuthPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<AppLayout />}>
          <Route index element={<Dashboard />} />
          <Route path="connections" element={<Connections />} />
          <Route path="plugins" element={<Plugins />} />
          <Route path="workspaces" element={<Workspaces />} />
          <Route path="query-lab" element={<QueryLab />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
