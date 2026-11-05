import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { organizationApi } from '../api/organizations';
import { useOrganizationContext } from '../hooks/useOrganizationContext';

export function MonitoringSourcesPage() {
  const queryClient = useQueryClient();
  const {
    organizations,
    currentOrganization,
    isLoading: orgLoading,
  } = useOrganizationContext();

  const [organizationId, setOrganizationId] = useState('');
  const [name, setName] = useState('');
  const [token, setToken] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const selectedId = organizationId || String(currentOrganization?.id || '');

  const selectedOrganization = organizations.find(
    (org) => String(org.id) === selectedId
  );

  const canManage =
    selectedOrganization?.role === 'company_admin' ||
    selectedOrganization?.role === 'platform_admin';

  const sourcesQuery = useQuery({
    queryKey: ['monitoring-sources', selectedId],
    queryFn: () => organizationApi.monitoringSources(selectedId),
    enabled: !!selectedId && !!canManage,
    retry: 1,
  });

  const sources = sourcesQuery.data?.sources || [];

  async function refresh() {
    await queryClient.invalidateQueries({
      queryKey: ['monitoring-sources', selectedId],
    });
  }

  async function createSource(event) {
    event.preventDefault();
    setBusy(true);
    setError('');
    setMessage('');
    setToken('');

    try {
      const result = await organizationApi.createMonitoringSource(
        selectedId,
        { name: name.trim(), source_type: 'docker' }
      );

      setToken(result.token);
      setName('');
      setMessage('Monitoring source created. Copy and save its token now.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not create source.');
    } finally {
      setBusy(false);
    }
  }

  async function rotateToken(source) {
    if (!window.confirm(`Rotate token for "${source.name}"? The old token will stop working.`)) return;

    setBusy(true);
    setError('');
    setMessage('');
    setToken('');

    try {
      const result = await organizationApi.rotateMonitoringToken(source.id);
      setToken(result.token);
      setMessage('Token rotated. Save the new token now.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not rotate token.');
    } finally {
      setBusy(false);
    }
  }

  async function disableSource(source) {
    if (!window.confirm(`Disable "${source.name}" and revoke its token?`)) return;

    setBusy(true);
    setError('');
    setMessage('');
    setToken('');

    try {
      await organizationApi.disableMonitoringSource(source.id);
      setMessage('Monitoring source disabled and token revoked.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not disable source.');
    } finally {
      setBusy(false);
    }
  }

  if (orgLoading) {
    return <div className="p-6 text-sm text-ink-400">Loading workspace...</div>;
  }

  if (!organizations.length) {
    return <div className="p-6">No organization available.</div>;
  }

  if (!canManage) {
    return <div className="p-6 text-sm text-ink-500">
      You do not have permission to manage monitoring sources.
    </div>;
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <header>
        <p className="text-xs font-semibold uppercase tracking-widest text-accent">
          OpsMind AI / Infrastructure
        </p>
        <h1 className="mt-2 text-2xl font-semibold text-ink-950 dark:text-white">
          Monitoring Sources
        </h1>
        <p className="mt-2 text-sm text-ink-500">
          Connect Docker monitoring agents to your company workspace.
        </p>
      </header>

      {organizations.length > 1 && (
        <section className="rounded-xl border border-ink-100 bg-white p-4 dark:border-white/10 dark:bg-[#1c1c1f]">
          <label className="text-sm font-medium">
            Select workspace
            <select
              className="db-input mt-2 w-full"
              value={selectedId}
              onChange={(event) => {
                setOrganizationId(event.target.value);
                setToken('');
                setMessage('');
                setError('');
              }}
            >
              {organizations.map((org) => (
                <option key={org.id} value={org.id}>
                  {org.name} — #{org.id}
                </option>
              ))}
            </select>
          </label>
        </section>
      )}

      {message && (
        <p role="status" className="rounded-lg bg-green-50 p-3 text-sm text-green-700">
          {message}
        </p>
      )}

      {error && (
        <p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">
          {error}
        </p>
      )}

      {token && (
        <section className="rounded-xl border border-amber-300 bg-amber-50 p-5">
          <h2 className="font-semibold text-amber-900">New monitoring token</h2>
          <p className="mt-1 text-sm text-amber-800">
            This token is shown only once. Save it securely; do not share it publicly.
          </p>
          <div className="mt-3 break-all rounded-lg bg-white p-3 font-mono text-sm text-ink-950">
            {token}
          </div>
          <button
            type="button"
            onClick={() => navigator.clipboard.writeText(token)}
            className="mt-3 rounded-lg bg-amber-700 px-4 py-2 text-sm text-white"
          >
            Copy token
          </button>
        </section>
      )}

      <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
        <h2 className="font-semibold">Add Docker monitoring source</h2>
        <form onSubmit={createSource} className="mt-4 flex flex-wrap gap-3">
          <input
            className="db-input min-w-0 flex-1"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="e.g. Production Docker Host"
            maxLength={150}
            required
          />
          <button
            type="submit"
            disabled={busy || !name.trim()}
            className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            {busy ? 'Please wait...' : 'Create source'}
          </button>
        </form>
      </section>

      <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold">Connected sources</h2>
          <span className="text-xs text-ink-400">{sources.length} sources</span>
        </div>

        {sourcesQuery.isLoading ? (
          <p className="mt-4 text-sm text-ink-400">Loading sources...</p>
        ) : sourcesQuery.error ? (
          <p className="mt-4 text-sm text-red-600">
            {sourcesQuery.error.message}
          </p>
        ) : sources.length === 0 ? (
          <p className="mt-4 text-sm text-ink-400">
            No monitoring sources connected yet.
          </p>
        ) : (
          <div className="mt-4 divide-y divide-ink-100 dark:divide-white/10">
            {sources.map((source) => (
              <div
                key={source.id}
                className="flex flex-wrap items-center justify-between gap-4 py-4"
              >
                <div>
                  <p className="text-sm font-medium">{source.name}</p>
                  <p className="mt-1 text-xs text-ink-400">
                    Docker · ID: {source.id}
                  </p>
                  <p className="mt-1 text-xs text-ink-500">
                    Last seen: {source.last_seen_at
                      ? new Date(source.last_seen_at).toLocaleString()
                      : 'Never'}
                  </p>
                  <span className={`mt-2 inline-flex rounded-full px-2 py-1 text-xs ${
                    source.is_enabled
                      ? 'bg-green-100 text-green-700'
                      : 'bg-red-100 text-red-700'
                  }`}>
                    {source.is_enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>

                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    disabled={busy || !source.is_enabled}
                    onClick={() => rotateToken(source)}
                    className="rounded-lg border px-3 py-2 text-xs disabled:opacity-40"
                  >
                    Rotate token
                  </button>
                  <button
                    type="button"
                    disabled={busy || !source.is_enabled}
                    onClick={() => disableSource(source)}
                    className="rounded-lg border border-red-200 px-3 py-2 text-xs text-red-600 disabled:opacity-40"
                  >
                    Disable
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}