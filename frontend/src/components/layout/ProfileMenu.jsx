
import { useEffect, useRef, useState } from 'react';
import {
  IconChevronDown,
  IconLogout,
  IconUserCircle,
  IconSettings,
  IconKeyboard,
  IconExternalLink,
} from '@tabler/icons-react';
import { useNavigate } from 'react-router-dom';
import { Avatar } from '../ui/Avatar';
import { clearAuth } from '../../api/client';

export function ProfileMenu() {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const rootRef = useRef(null);

  let user = null;

  try {
    user = JSON.parse(localStorage.getItem('opsmind_user') || 'null');
  } catch {
    user = null;
  }

  useEffect(() => {
    if (!open) return;

    function handleOutsideClick(event) {
      if (rootRef.current && !rootRef.current.contains(event.target)) {
        setOpen(false);
      }
    }

    function handleKeyDown(event) {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    }

    document.addEventListener('mousedown', handleOutsideClick);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('mousedown', handleOutsideClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  function handleSignOut() {
    setOpen(false);
    clearAuth();
    navigate('/login', { replace: true });
  }

  if (!user) return null;

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        data-testid="profile-trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className={`h-8 inline-flex items-center gap-1.5 pl-0.5 pr-1.5 rounded-full transition
          ${
            open
              ? 'bg-ink-50 dark:bg-white/10'
              : 'hover:bg-ink-50 dark:hover:bg-white/5'
          }`}
      >
        <span className="relative">
          <Avatar name={user.name || 'User'} size={26} />
          <span className="absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full bg-success ring-2 ring-white dark:ring-[#1c1c1f]" />
        </span>

        <IconChevronDown
          size={13}
          stroke={1.8}
          className={`text-ink-400 transition-transform ${
            open ? 'rotate-180' : ''
          }`}
        />
      </button>

      {open && (
        <div
          role="menu"
          data-testid="profile-menu"
          className="absolute right-0 top-full mt-1.5 w-64 z-40
            rounded-card border border-ink-100 dark:border-white/10
            bg-white dark:bg-[#1c1c1f]
            shadow-[0_10px_40px_-12px_rgba(0,0,0,0.18)] dark:shadow-[0_10px_40px_-12px_rgba(0,0,0,0.6)]
            overflow-hidden"
        >
          <div className="px-3 py-3 flex items-center gap-2.5 border-b border-ink-100 dark:border-white/10">
            <Avatar name={user.name || 'User'} size={32} />

            <div className="min-w-0">
              <div className="text-[13px] font-medium leading-tight truncate">
                {user.name || 'User'}
              </div>

              <div className="text-[11px] text-ink-400 font-mono leading-tight truncate">
                {user.email || ''}
              </div>
            </div>
          </div>

          <nav className="py-1 text-[13px]">
            <MenuItem
              icon={IconUserCircle}
              label="View profile"
              onClick={() => setOpen(false)}
            />

            <MenuItem
              icon={IconSettings}
              label="Settings"
              onClick={() => setOpen(false)}
            />

            <MenuItem
              icon={IconKeyboard}
              label="Keyboard shortcuts"
              meta={<KeyHint k="?" />}
              onClick={() => setOpen(false)}
            />

            <div className="my-1 h-px bg-ink-100 dark:bg-white/10" />

            <MenuItem
              icon={IconExternalLink}
              label="GitHub repo"
              onClick={() => {
                setOpen(false);
                window.open(
                  'https://github.com/LondheShubham153/devboard',
                  '_blank'
                );
              }}
            />

            <div className="my-1 h-px bg-ink-100 dark:bg-white/10" />

            <MenuItem
              icon={IconLogout}
              label="Sign out"
              danger
              onClick={handleSignOut}
              meta={<KeyHint k="⇧⌘Q" />}
            />
          </nav>
        </div>
      )}
    </div>
  );
}

function MenuItem({ icon: Icon, label, onClick, meta, danger }) {
  return (
    <button
      role="menuitem"
      type="button"
      onClick={onClick}
      className={`w-full px-3 h-8 inline-flex items-center gap-2.5 text-left transition
        ${
          danger
            ? 'text-[#A32D2D] hover:bg-danger-bg'
            : 'hover:bg-ink-50 dark:hover:bg-white/5'
        }`}
    >
      <Icon size={14} stroke={1.6} className="shrink-0" />
      <span className="flex-1 truncate">{label}</span>
      {meta}
    </button>
  );
}

function KeyHint({ k }) {
  return (
    <span className="font-mono text-[10px] text-ink-400 px-1.5 h-[18px] inline-flex items-center
      rounded border border-ink-100 dark:border-white/10 bg-ink-50 dark:bg-white/5">
      {k}
    </span>
  );
}
