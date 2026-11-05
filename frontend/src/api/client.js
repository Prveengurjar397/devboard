const TOKEN_KEY = 'opsmind_token';

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function saveAuth(data) {
  if (data?.token) localStorage.setItem(TOKEN_KEY, data.token);
  if (data?.user) localStorage.setItem('opsmind_user', JSON.stringify(data.user));
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem('opsmind_user');
}

async function request(path, { method = 'GET', body, headers = {} } = {}) {
  const token = getToken();
  const res = await fetch(path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }

  if (!res.ok) {
    const error = new Error(data?.error || `HTTP ${res.status}`);
    error.status = res.status;
    error.data = data;
    throw error;
  }
  return data;
}

export const api = {
  get: (path) => request(path),
  post: (path, body) => request(path, { method: 'POST', body }),
  patch: (path, body) => request(path, { method: 'PATCH', body }),
  delete: (path) => request(path, { method: 'DELETE' }),
};

export const authApi = {
  acceptInvitation: (token, name, password) =>
   api.post('/api/invitations/accept', { token, name, password }),
  login: (email, password) => api.post('/api/auth/login', { email, password }),
  register: (name, email, password, companyName) =>
    api.post('/api/auth/register', {
      name,
      email,
      password,
      company_name: companyName,
    }),
};
