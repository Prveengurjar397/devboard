import { useCallback, useEffect, useState } from 'react';
import { incidentApi } from '../api/incidents';
import { Link } from 'react-router-dom';

const SEVERITIES = ['critical', 'high', 'medium', 'low'];

function StatCard({ label, value, color }) {
  return (
    <div className="rounded-xl border border-ink-100 dark:border-white/10 bg-white dark:bg-[#1c1c1f] p-4">
      <p className="text-sm text-ink-400">{label}</p>
      <p className={`mt-2 text-3xl font-semibold ${color}`}>{value}</p>
    </div>
  );
}

function IncidentBadge({ value }) {
  const styles = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-300',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/15 dark:text-yellow-300',
    low: 'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300',
    open: 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    acknowledged: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/15 dark:text-yellow-300',
    resolved: 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-300',
  };

  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-medium ${styles[value] || 'bg-ink-100 text-ink-600'}`}>
      {value}
    </span>
  );
}

export function IncidentsPage() {
  const [incidents, setIncidents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadIncidents = useCallback(async () => {
    try {
      setError('');
      const data = await incidentApi.list();
      setIncidents(data.incidents || []);
    } catch (err) {
      setError(err.message || 'Unable to load incidents');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadIncidents();
  }, [loadIncidents]);

  async function updateStatus(id, status) {
    try {
      await incidentApi.update(id, { status });
      await loadIncidents();
    } catch (err) {
      setError(err.message || 'Unable to update incident');
    }
  }

  const count = (severity) =>
    incidents.filter((item) => item.severity === severity).length;

  const openCount = incidents.filter((item) => item.status === 'open').length;
  const acknowledgedCount = incidents.filter((item) => item.status === 'acknowledged').length;
  const resolvedCount = incidents.filter((item) => item.status === 'resolved').length;

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-widest text-accent">
            OpsMind AI
          </p>
          <h1 className="mt-1 text-2xl font-semibold text-ink-950 dark:text-white">
            Incident Overview
          </h1>
          <p className="mt-1 text-sm text-ink-400">
            Monitor and manage your DevOps incidents.
          </p>
        </div>

        <button
          onClick={loadIncidents}
          disabled={loading}
          className="rounded-lg border border-ink-200 dark:border-white/10 px-4 py-2 text-sm hover:bg-ink-50 dark:hover:bg-white/5 disabled:opacity-50"
        >
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard label="Total incidents" value={incidents.length} color="text-ink-950 dark:text-white" />
        <StatCard label="Critical" value={count('critical')} color="text-red-600" />
        <StatCard label="High" value={count('high')} color="text-orange-600" />
        <StatCard label="Medium" value={count('medium')} color="text-yellow-600" />
        <StatCard label="Low" value={count('low')} color="text-blue-600" />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <StatCard label="Open" value={openCount} color="text-red-600" />
        <StatCard label="Acknowledged" value={acknowledgedCount} color="text-yellow-600" />
        <StatCard label="Resolved" value={resolvedCount} color="text-green-600" />
      </div>

      <section className="overflow-hidden rounded-xl border border-ink-100 dark:border-white/10 bg-white dark:bg-[#1c1c1f]">
        <div className="border-b border-ink-100 dark:border-white/10 p-4">
          <h2 className="font-semibold text-ink-950 dark:text-white">
            Recent incidents
          </h2>
          <p className="mt-1 text-xs text-ink-400">
            Incidents retrieved from the DevBoard backend.
          </p>
        </div>

        {loading ? (
          <p className="p-6 text-sm text-ink-400">Loading incidents...</p>
        ) : error ? (
          <div className="p-6">
            <p className="text-sm text-red-600">{error}</p>
            <button onClick={loadIncidents} className="mt-3 text-sm text-accent">
              Try again
            </button>
          </div>
        ) : incidents.length === 0 ? (
          <p className="p-6 text-sm text-ink-400">No incidents found.</p>
        ) : (
          <div className="divide-y divide-ink-100 dark:divide-white/10">
            {incidents.map((incident) => (
              <article key={incident.id} className="space-y-3 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="font-medium text-ink-950 dark:text-white">
                      <Link
                        to={`/incidents/${incident.id}`}
                        className="hover:text-accent hover:underline"
                      >
                        {incident.title}
                      </Link>
                    </h3>
                    <p className="mt-1 text-xs text-ink-400">
                      Incident #{incident.id}
                      {incident.container_name && ` · ${incident.container_name}`}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <IncidentBadge value={incident.severity} />
                    <IncidentBadge value={incident.status} />
                  </div>
                </div>

                {incident.description && (
                  <p className="text-sm text-ink-600 dark:text-ink-300">
                    {incident.description}
                  </p>
                )}

                {incident.root_cause && (
                  <div className="rounded-lg bg-ink-50 dark:bg-white/5 p-3">
                    <p className="text-xs font-semibold text-ink-500">Root cause</p>
                    <p className="mt-1 text-sm text-ink-700 dark:text-ink-200">
                      {incident.root_cause}
                    </p>
                  </div>
                )}

                {incident.suggested_fix && (
                  <div className="rounded-lg bg-ink-50 dark:bg-white/5 p-3">
                    <p className="text-xs font-semibold text-ink-500">Suggested fix</p>
                    <p className="mt-1 text-sm text-ink-700 dark:text-ink-200">
                      {incident.suggested_fix}
                    </p>
                  </div>
                )}

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="text-xs text-ink-400">
                    {new Date(incident.created_at).toLocaleString()}
                  </span>

                  <div className="flex gap-2">
                    {incident.status === 'open' && (
                      <button
                        onClick={() => updateStatus(incident.id, 'acknowledged')}
                        className="rounded-lg border border-ink-200 dark:border-white/10 px-3 py-1.5 text-xs hover:bg-ink-50 dark:hover:bg-white/5"
                      >
                        Acknowledge
                      </button>
                    )}

                    {incident.status !== 'resolved' && (
                      <button
                        onClick={() => updateStatus(incident.id, 'resolved')}
                        className="rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-white hover:opacity-90"
                      >
                        Resolve
                      </button>
                    )}
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
