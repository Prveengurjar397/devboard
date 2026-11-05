import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { authApi, saveAuth } from '../api/client';
import { Button } from '../components/ui/Button';

export function InviteAcceptPage() {
  const [params] = useSearchParams();
  const token = params.get('token') || '';
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();

  async function submit(event) {
    event.preventDefault();
    setError('');
    setBusy(true);

    try {
      const result = await authApi.acceptInvitation(token, name, password);
      saveAuth(result);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err.message || 'Unable to accept invitation.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-5 bg-ink-50 dark:bg-[#111113]">
      <section className="db-card w-full max-w-md rounded-xl p-7">
        <p className="text-xs uppercase tracking-widest text-ink-400 mb-2">OpsMind AI</p>
        <h1 className="text-2xl font-semibold mb-2">Join your company</h1>
        <p className="text-sm text-ink-500 mb-6">
          Accept your invitation using your email account.
        </p>

        {!token ? (
          <p role="alert" className="text-sm text-red-600">
            Invitation token is missing. Please use the complete invitation link.
          </p>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <label className="block text-sm">
              Your name
              <input
                className="db-input mt-1 w-full"
                value={name}
                onChange={(event) => setName(event.target.value)}
                maxLength={100}
                placeholder="Required for a new account"
              />
            </label>

            <label className="block text-sm">
              Password
              <input
                className="db-input mt-1 w-full"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
                minLength={8}
                maxLength={72}
                autoComplete="current-password"
              />
            </label>

            {error && <p role="alert" className="text-sm text-red-600">{error}</p>}

            <Button type="submit" disabled={busy}>
              {busy ? 'Joining...' : 'Accept invitation'}
            </Button>
          </form>
        )}
      </section>
    </main>
  );
}
