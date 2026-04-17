import { Routes, Route } from 'react-router-dom';
import { AppLayout } from './layout/AppLayout';
import { Dashboard } from './pages/Dashboard';
import { Connections } from './pages/Connections';
import { Plugins } from './pages/Plugins';
import { Workspaces } from './pages/Workspaces';
import { QueryLab } from './pages/QueryLab';

function App() {
  return (
    <Routes>
      <Route path="/" element={<AppLayout />}>
        <Route index element={<Dashboard />} />
        <Route path="connections" element={<Connections />} />
        <Route path="plugins" element={<Plugins />} />
        <Route path="workspaces" element={<Workspaces />} />
        <Route path="query-lab" element={<QueryLab />} />
      </Route>
    </Routes>
  );
}

export default App;
