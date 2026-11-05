import { Routes, Route, Navigate } from 'react-router-dom';
import { AppShell } from './components/layout/AppShell';
import { DashboardPage } from './pages/DashboardPage';
import { ProjectPage } from './pages/ProjectPage';
import { IncidentsPage } from './pages/IncidentsPage';
import { IncidentDetailPage } from './pages/IncidentDetailPage';
import { AuthPage } from './pages/AuthPage';
import { OrganizationSettingsPage } from './pages/OrganizationSettingsPage';
import { getToken } from './api/client';
import { MonitoringSourcesPage } from './pages/MonitoringSourcesPage';
import { ChatbotPage } from './pages/ChatbotPage';
import { InviteAcceptPage } from './pages/InviteAcceptPage';


function RequireAuth({ children }) {
  return getToken() ? children : <Navigate to="/login" replace />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={
        getToken() ? <Navigate to="/" replace /> : <AuthPage />} />
      <Route path="/invite/accept" element={<InviteAcceptPage />} />
      <Route element={<RequireAuth><AppShell /></RequireAuth>}>
        <Route index element={<DashboardPage />} />
        <Route path="/projects/:id" element={<ProjectPage />} />
        <Route path="/incidents" element={<IncidentsPage />} />
        <Route path="/chatbot" element={<ChatbotPage />} />
        <Route path="/incidents/:id" element={<IncidentDetailPage />} />
        <Route path="/organization" element={<OrganizationSettingsPage />} />
        <Route path="/monitoring-sources" element={<MonitoringSourcesPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
