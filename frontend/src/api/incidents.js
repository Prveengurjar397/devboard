import { api } from './client';

export const incidentApi = {
  list: () => api.get('/api/incidents'),
  get: (id) => api.get(`/api/incidents/${id}`),
  create: (data) => api.post('/api/incidents', data),
  update: (id, data) => api.patch(`/api/incidents/${id}`, data),
  events: (id) => api.get(`/api/incidents/${id}/events`),
};
