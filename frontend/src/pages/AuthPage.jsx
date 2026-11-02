import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { authApi, saveAuth } from '../api/client';
import { Button } from '../components/ui/Button';

export function AuthPage() {
  const [mode, setMode] = useState('login');
  const [name, setName] = useState('');
  const [companyName, setCompanyName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();

  async function submit(e) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const result = mode === 'login'
        ? await authApi.login(email, password)
        : await authApi.register(name, email, password, companyName);
      saveAuth(result);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err.message || 'Authentication failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-5 bg-ink-50 dark:bg-[#111113]">
      <section className="db-card w-full max-w-md rounded-xl p-7">
        <p className="text-xs uppercase tracking-widest text-ink-400 mb-2">OpsMind AI</p>
        <h1 className="text-2xl font-semibold mb-2">
          {mode === 'login' ? 'Welcome back' : 'Create your workspace'}
        </h1>
        <p className="text-sm text-ink-500 mb-6">
          {mode === 'login' ? 'Sign in to continue.' : 'Register to access your OpsMind workspace.'}
        </p>

        <form onSubmit={submit} className="space-y-4">
          {mode === 'register' && (
            <>
              <label className="block text-sm">
                Your Name
                <input className="db-input w-full mt-1" value={name}
                  onChange={e => setName(e.target.value)} required maxLength={100} />
              </label>
              <label className="block text-sm">
                Company Name
                <input className="db-input w-full mt-1" value={companyName}
                  onChange={e => setCompanyName(e.target.value)}
                  required maxLength={150} />
              </label>
            </>
          )}
          <label className="block text-sm">
            Email
            <input className="db-input w-full mt-1" type="email" value={email}
              onChange={e => setEmail(e.target.value)} required />
          </label>
          <label className="block text-sm">
            Password
            <input className="db-input w-full mt-1" type="password" value={password}
              onChange={e => setPassword(e.target.value)} required minLength={8} />
          </label>

          {error && <p role="alert" className="text-sm text-danger">{error}</p>}

          <Button type="submit" disabled={busy}>
            {busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
          </Button>
        </form>

        <button className="mt-5 text-sm text-accent" type="button"
          onClick={() => { setError(''); setMode(mode === 'login' ? 'register' : 'login'); }}>
          {mode === 'login' ? 'New here? Create account' : 'Already registered? Sign in'}
        </button>
      </section>
    </main>
  );
}
