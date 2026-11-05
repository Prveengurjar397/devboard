import { api } from './client';

export const organizationApi = {
  list: () => api.get('/api/organizations'),
  members: (id) => api.get(`/api/organizations/${id}/members`),
  addMember: (id, data) => api.post(`/api/organizations/${id}/members`, data),
  createInvitation: (id, data) =>
  api.post(`/api/organizations/${id}/invitations`, data),
  updateMember: (id, userId, data) =>
    api.patch(`/api/organizations/${id}/members/${userId}`, data),
  removeMember: (id, userId) =>
    api.delete(`/api/organizations/${id}/members/${userId}`),
    monitoringSources: (id) =>
    api.get(`/api/organizations/${id}/monitoring-sources`),

  createMonitoringSource: (id, data) =>
    api.post(`/api/organizations/${id}/monitoring-sources`, data),

  rotateMonitoringToken: (id) =>
    api.post(`/api/monitoring-sources/${id}/rotate-token`, {}),

  disableMonitoringSource: (id) =>
    api.delete(`/api/monitoring-sources/${id}`),
};
