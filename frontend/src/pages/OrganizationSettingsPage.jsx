import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { organizationApi } from '../api/organizations';
import { useOrganizationContext } from '../hooks/useOrganizationContext';

export function OrganizationSettingsPage() {
  const queryClient = useQueryClient();
  const {
    organizations,
    isPlatformAdmin,
    isLoading: organizationsLoading,
    error: organizationsError,
  } = useOrganizationContext();

  const [selectedId, setSelectedId] = useState('');
  const [email, setEmail] = useState('');
  const [role, setRole] = useState('developer');
  const [incidentAccess, setIncidentAccess] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!selectedId && organizations.length) {
      setSelectedId(String(organizations[0].id));
    }
  }, [organizations, selectedId]);

  const selectedOrganization = organizations.find(
    (organization) => String(organization.id) === String(selectedId)
  );

  const membersQuery = useQuery({
    queryKey: ['organization-members', selectedId],
    queryFn: () => organizationApi.members(selectedId),
    enabled: !!selectedId && !!selectedOrganization &&
      (selectedOrganization.role === 'company_admin' || isPlatformAdmin),
  });

  const members = membersQuery.data?.members || [];
  const canManage = selectedOrganization &&
    (selectedOrganization.role === 'company_admin' || isPlatformAdmin);

  async function refresh() {
    await queryClient.invalidateQueries({
      queryKey: ['organization-members', selectedId],
    });
    await queryClient.invalidateQueries({ queryKey: ['organizations'] });
  }

  async function addMember(event) {
    event.preventDefault();
    setError('');
    setMessage('');
    setBusy(true);

    try {
      await organizationApi.addMember(selectedId, {
        email: email.trim(),
        role,
        can_access_incidents: incidentAccess,
      });
      setEmail('');
      setMessage('Member added or updated successfully.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Unable to add member.');
    } finally {
      setBusy(false);
    }
  }

  async function changeIncidentAccess(member, checked) {
    setError('');
    setMessage('');
    try {
      await organizationApi.updateMember(selectedId, member.user_id, {
        can_access_incidents: checked,
      });
      setMessage('Incident access updated.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Unable to update permission.');
    }
  }

  async function removeMember(member) {
    if (!window.confirm(`Remove ${member.email} from this company?`)) return;

    setError('');
    setMessage('');
    try {
      await organizationApi.removeMember(selectedId, member.user_id);
      setMessage('Member removed from company.');
      await refresh();
    } catch (err) {
      setError(err.message || 'Unable to remove member.');
    }
  }

  if (organizationsLoading) {
    return <div className="p-6 text-sm text-ink-400">Loading organizations...</div>;
  }

  if (organizationsError) {
    return <div className="p-6 text-sm text-red-600">{organizationsError.message}</div>;
  }

  if (!organizations.length) {
    return (
      <div className="mx-auto max-w-4xl p-6">
        <h1 className="text-2xl font-semibold">Organization</h1>
        <p className="mt-3 text-sm text-ink-500">
          This account is not currently a member of a company.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <header>
        <p className="text-xs font-semibold uppercase tracking-widest text-accent">
          OpsMind AI / Workspace
        </p>
        <h1 className="mt-2 text-2xl font-semibold text-ink-950 dark:text-white">
          Company management
        </h1>
        <p className="mt-2 text-sm text-ink-500">
          Manage company members and their access to Incidents.
        </p>
      </header>

      {isPlatformAdmin && (
        <section className="rounded-xl border border-ink-100 bg-white p-4 dark:border-white/10 dark:bg-[#1c1c1f]">
          <label className="block text-sm font-medium">
            Select company
            <select
              className="db-input mt-2 w-full"
              value={selectedId}
              onChange={(event) => setSelectedId(event.target.value)}
            >
              {organizations.map((organization) => (
                <option key={organization.id} value={organization.id}>
                  {organization.name} — #{organization.id}
                </option>
              ))}
            </select>
          </label>
        </section>
      )}

      {selectedOrganization && (
        <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
          <h2 className="font-semibold text-ink-950 dark:text-white">
            {selectedOrganization.name}
          </h2>
          <p className="mt-1 text-xs text-ink-400">
            Organization ID: {selectedOrganization.id} · Role: {selectedOrganization.role}
          </p>
        </section>
      )}

      {message && <p role="status" className="text-sm text-green-600">{message}</p>}
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}

      {canManage ? (
        <>
          <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
            <h2 className="font-semibold">Add a registered user</h2>
            <p className="mt-1 text-xs text-ink-400">
              The user must already have an OpsMind account.
            </p>

            <form onSubmit={addMember} className="mt-4 grid gap-3 md:grid-cols-2">
              <label className="text-sm">
                User email
                <input
                  className="db-input mt-1 w-full"
                  type="email"
                  required
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  placeholder="user@example.com"
                />
              </label>

              <label className="text-sm">
                Company role
                <select
                  className="db-input mt-1 w-full"
                  value={role}
                  onChange={(event) => setRole(event.target.value)}
                >
                  <option value="developer">Developer</option>
                  <option value="tester">Tester</option>
                  <option value="devops">DevOps</option>
                </select>
              </label>

              <label className="flex items-center gap-2 text-sm md:col-span-2">
                <input
                  type="checkbox"
                  checked={incidentAccess}
                  onChange={(event) => setIncidentAccess(event.target.checked)}
                />
                Allow access to Incidents
              </label>

              <button
                type="submit"
                disabled={busy || !selectedId}
                className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50 md:col-span-2 md:w-fit"
              >
                {busy ? 'Adding...' : 'Add member'}
              </button>
            </form>
          </section>

          <section className="rounded-xl border border-ink-100 bg-white p-5 dark:border-white/10 dark:bg-[#1c1c1f]">
            <div className="flex items-center justify-between">
              <h2 className="font-semibold">Company members</h2>
              <span className="text-xs text-ink-400">{members.length} members</span>
            </div>

            {membersQuery.isLoading ? (
              <p className="mt-4 text-sm text-ink-400">Loading members...</p>
            ) : membersQuery.error ? (
              <p className="mt-4 text-sm text-red-600">{membersQuery.error.message}</p>
            ) : members.length === 0 ? (
              <p className="mt-4 text-sm text-ink-400">No members found.</p>
            ) : (
              <div className="mt-4 divide-y divide-ink-100 dark:divide-white/10">
                {members.map((member) => (
                  <div
                    key={member.user_id}
                    className="flex flex-wrap items-center justify-between gap-4 py-4"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{member.name || 'User'}</p>
                      <p className="truncate text-xs text-ink-400">{member.email}</p>
                      <p className="mt-1 text-xs text-ink-500">
                        Role: {member.role}
                      </p>
                    </div>

                    <div className="flex flex-wrap items-center gap-4">
                      <label className="flex items-center gap-2 text-xs">
                        <input
                          type="checkbox"
                          checked={!!member.can_access_incidents}
                          disabled={member.role === 'company_admin'}
                          onChange={(event) =>
                            changeIncidentAccess(member, event.target.checked)
                          }
                        />
                        Incident access
                      </label>

                      {member.role !== 'company_admin' && (
                        <button
                          type="button"
                          onClick={() => removeMember(member)}
                          className="text-xs font-medium text-red-600 hover:underline"
                        >
                          Remove
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </>
      ) : (
        <p className="text-sm text-ink-500">
          You can view your company workspace, but only the Company Owner can manage members.
        </p>
      )}
    </div>
  );
}
