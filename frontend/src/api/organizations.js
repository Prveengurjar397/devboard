import { api } from './client';

export const organizationApi = {
  list: () => api.get('/api/organizations'),
  members: (id) => api.get(`/api/organizations/${id}/members`),
  addMember: (id, data) => api.post(`/api/organizations/${id}/members`, data),
  updateMember: (id, userId, data) =>
    api.patch(`/api/organizations/${id}/members/${userId}`, data),
  removeMember: (id, userId) =>
    api.delete(`/api/organizations/${id}/members/${userId}`),
};
