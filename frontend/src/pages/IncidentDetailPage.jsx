import { useEffect, useState } from 'react';
import { useTasks } from '../hooks/useTasks';
import { Link, useParams } from 'react-router-dom';
import { incidentApi } from '../api/incidents';

function Panel({ title, children }) {
  return (
    <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
      <h2 className="mb-3 font-semibold text-ink-950 dark:text-white">{title}</h2>
      {children}
    </section>
  );
}

function StatusBadge({ value }) {
  const colors = {
    critical: 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    high: 'bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-300',
    medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/15 dark:text-yellow-300',
    low: 'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300',
    open: 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    acknowledged: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/15 dark:text-yellow-300',
    resolved: 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-300',
    in_progress: 'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300',
  };

  return (
    <span className={`rounded-full px-3 py-1 text-xs font-semibold ${colors[value] || 'bg-ink-100 text-ink-600'}`}>
      {value || 'unknown'}
    </span>
  );
}

export function IncidentDetailPage() {
  const { id } = useParams();
  const [incident, setIncident] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [updating, setUpdating] = useState(false);
  const [copied, setCopied] = useState('');
  const [events, setEvents] = useState([]);
    const { data: taskData, isLoading: tasksLoading } = useTasks(1);

  const linkedTask = (taskData?.tasks || []).find(
    (task) => Number(task.id) === Number(incident?.task_id)
  );
  const [eventsLoading, setEventsLoading] = useState(true);
  const [eventsError, setEventsError] = useState('');

  async function loadIncident() {
    try {
      setError('');
      const data = await incidentApi.get(id);
      setIncident(data.incident || data);
    } catch (err) {
      setError(err.message || 'Unable to load incident');
    } finally {
      setLoading(false);
    }
  }

  async function loadEvents() {
    try {
      setEventsError('');
      setEventsLoading(true);
      const data = await incidentApi.events(id);
      setEvents(data.events || []);
    } catch (err) {
      setEventsError(err.message || 'Unable to load activity timeline');
    } finally {
      setEventsLoading(false);
    }
  }

  useEffect(() => {
    loadIncident();
    loadEvents();
  }, [id]);

  async function changeStatus(status) {
    try {
      setUpdating(true);
      await incidentApi.update(id, { status });
      await loadIncident();
      await loadEvents();
    } catch (err) {
      setError(err.message || 'Unable to update incident');
    } finally {
      setUpdating(false);
    }
  }

  async function copyText(label, value) {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      setTimeout(() => setCopied(''), 1800);
    } catch {
      setError('Clipboard access is unavailable in this browser.');
    }
  }

  if (loading) {
    return <div className="p-8 text-sm text-ink-400">Loading incident details...</div>;
  }

  if (error && !incident) {
    return (
      <div className="mx-auto max-w-5xl p-6">
        <Link to="/incidents" className="text-sm text-accent">← Back to incidents</Link>
        <p className="mt-5 text-red-600">{error}</p>
        <button onClick={loadIncident} className="mt-3 rounded-lg border px-4 py-2 text-sm">
          Try again
        </button>
      </div>
    );
  }

  if (!incident) return null;

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <Link to="/incidents" className="text-sm text-accent hover:underline">
        ← Back to incidents
      </Link>

      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-widest text-accent">
            OpsMind AI / Incident #{incident.id}
          </p>
          <h1 className="mt-2 text-2xl font-semibold text-ink-950 dark:text-white">
            {incident.title}
          </h1>
          <p className="mt-2 text-sm text-ink-400">
            {incident.container_name || 'Container not specified'}
            {' · '}
            {incident.created_at ? new Date(incident.created_at).toLocaleString() : 'Time unavailable'}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <StatusBadge value={incident.severity} />
          <StatusBadge value={incident.status} />
        </div>
      </header>

      {error && <p className="text-sm text-red-600">{error}</p>}
            <Panel title="Linked investigation task">
        {incident.task_id ? (
          tasksLoading ? (
            <p className="text-sm text-ink-400">Loading linked task...</p>
          ) : linkedTask ? (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-semibold text-ink-950 dark:text-white">
                  Task #{linkedTask.id}
                </span>
                <StatusBadge value={linkedTask.status} />
              </div>

              <p className="text-sm text-ink-700 dark:text-ink-300">
                {linkedTask.title}
              </p>

              <Link
                to="/projects/1"
                className="inline-block text-sm font-medium text-accent hover:underline"
              >
                Open Tasks Board →
              </Link>
            </div>
          ) : (
            <p className="text-sm text-ink-400">
              Linked task #{incident.task_id} exists, but its details are unavailable.
            </p>
          )
        ) : (
          <p className="text-sm text-ink-400">
            No investigation task is linked to this incident.
          </p>
        )}
      </Panel>
      <div className="grid gap-4 md:grid-cols-2">
        <Panel title="Problem description">
          <p className="whitespace-pre-wrap text-sm leading-6 text-ink-600 dark:text-ink-300">
            {incident.description || 'No description was provided for this incident.'}
          </p>
        </Panel>

        <Panel title="AI root cause analysis">
          <p className="whitespace-pre-wrap text-sm leading-6 text-ink-600 dark:text-ink-300">
            {incident.root_cause || 'Root cause analysis is not available yet.'}
          </p>
          <p className="mt-3 text-xs text-ink-400">
            Analysis is based on the information recorded for this incident. Verify before taking action.
          </p>
        </Panel>

        <Panel title="Suggested resolution">
          <p className="whitespace-pre-wrap text-sm leading-6 text-ink-600 dark:text-ink-300">
            {incident.suggested_fix || 'No resolution suggestion is available yet.'}
          </p>
          {incident.suggested_fix && (
            <button
              onClick={() => copyText('fix', incident.suggested_fix)}
              className="mt-4 rounded-lg border border-ink-200 px-3 py-2 text-xs dark:border-white/10"
            >
              {copied === 'fix' ? 'Copied' : 'Copy suggestion'}
            </button>
          )}
        </Panel>

        <Panel title="Incident evidence / Logs">
          <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-ink-950 p-4 text-xs leading-5 text-green-300">
            {incident.logs || 'No logs attached to this incident.'}
          </pre>
          {incident.logs && (
            <button
              onClick={() => copyText('logs', incident.logs)}
              className="mt-3 rounded-lg border border-ink-200 px-3 py-2 text-xs dark:border-white/10"
            >
              {copied === 'logs' ? 'Copied' : 'Copy logs'}
            </button>
          )}
        </Panel>
      </div>


      <Panel title="Activity timeline">
        {eventsLoading ? (
          <p className="text-sm text-ink-400">Loading activity...</p>
        ) : eventsError ? (
          <p className="text-sm text-red-600">{eventsError}</p>
        ) : events.length === 0 ? (
          <p className="text-sm text-ink-400">
            No activity recorded yet. New incident and status events will appear here.
          </p>
        ) : (
          <ol className="space-y-4">
            {events.map((event) => (
              <li key={event.id} className="flex gap-3">
                <span className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full bg-accent" />
                <div className="min-w-0">
                  <p className="text-sm font-medium text-ink-800 dark:text-white">
                    {event.message}
                  </p>
                  <p className="mt-1 text-xs text-ink-400">
                    {event.event_type.replaceAll('_', ' ')}
                    {' · '}
                    {event.actor_type}
                    {' · '}
                    {new Date(event.created_at).toLocaleString()}
                  </p>
                </div>
              </li>
            ))}
          </ol>
        )}
      </Panel>

      <Panel title="Incident response">
        <p className="mb-4 text-sm text-ink-500 dark:text-ink-400">
          Review the evidence and suggested resolution before changing the incident status.
          No recovery command is executed by this page.
        </p>
        <div className="flex flex-wrap gap-3">
          {incident.status === 'open' && (
            <button
              disabled={updating}
              onClick={() => changeStatus('acknowledged')}
              className="rounded-lg border border-ink-200 px-4 py-2 text-sm hover:bg-ink-50 disabled:opacity-50 dark:border-white/10 dark:hover:bg-white/5"
            >
              {updating ? 'Updating...' : 'Acknowledge incident'}
            </button>
          )}
          {incident.status !== 'resolved' && (
            <button
              disabled={updating}
              onClick={() => changeStatus('resolved')}
              className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              {updating ? 'Updating...' : 'Mark as resolved'}
            </button>
          )}
        </div>
      </Panel>
    </div>
  );
}
