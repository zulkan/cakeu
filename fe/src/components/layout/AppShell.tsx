import { useEffect, useState } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import {
  ArrowLeftRight,
  BarChart3,
  KeyRound,
  LayoutDashboard,
  LogOut,
  Menu,
  MessageSquare,
  Repeat,
  Tags,
  Wallet,
  X,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useAuthStore } from '../../stores/authStore';
import { displayName, initials } from '../../lib/format';
import { cn } from '../../lib/cn';
import { BrandMark } from '../BrandMark';
import { Toaster } from '../ui/Toaster';

interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

const navItems: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Accounts', icon: Wallet },
  { to: '/categories', label: 'Categories', icon: Tags },
  { to: '/recurring', label: 'Recurring', icon: Repeat },
  { to: '/reports', label: 'Reports', icon: BarChart3 },
  { to: '/chat', label: 'Assistant', icon: MessageSquare },
];

function NavItems({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <nav aria-label="Main navigation" className="space-y-1">
      {navItems.map(({ to, label, icon: Icon }) => (
        <NavLink
          key={to}
          to={to}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold transition-colors',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
              isActive
                ? 'bg-brand-50 text-brand-800 ring-1 ring-brand-200'
                : 'text-stone-600 hover:bg-warm-100 hover:text-stone-900'
            )
          }
        >
          <Icon className="h-5 w-5" aria-hidden="true" />
          {label}
        </NavLink>
      ))}
    </nav>
  );
}

function UserCard({
  onLogout,
  onNavigate,
}: {
  onLogout: () => void;
  onNavigate?: () => void;
}) {
  const user = useAuthStore((state) => state.user);

  if (!user) return null;

  return (
    <div className="rounded-2xl border border-warm-200 bg-warm-50/70 p-3">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-brand-500 text-sm font-bold text-white">
          {initials(user.email)}
        </div>
        <div className="min-w-0">
          <p className="truncate text-sm font-bold text-stone-900">
            {displayName(user.email)}
          </p>
          <p className="truncate text-xs text-stone-500">{user.email}</p>
        </div>
      </div>
      <NavLink
        to="/change-password"
        onClick={onNavigate}
        className={({ isActive }) =>
          cn(
            'mt-3 flex w-full items-center justify-center gap-2 rounded-xl border px-3 py-2 text-sm font-semibold transition-colors',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
            isActive
              ? 'border-brand-300 bg-brand-50 text-brand-800'
              : 'border-warm-200 bg-surface-card text-stone-600 hover:border-brand-300 hover:bg-brand-50 hover:text-brand-800'
          )
        }
      >
        <KeyRound className="h-4 w-4" aria-hidden="true" />
        Change password
      </NavLink>
      <button
        type="button"
        onClick={onLogout}
        className="mt-2 flex w-full items-center justify-center gap-2 rounded-xl border border-warm-200 bg-surface-card px-3 py-2 text-sm font-semibold text-stone-600 transition-colors hover:border-expense/30 hover:bg-expense-light/40 hover:text-expense focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
      >
        <LogOut className="h-4 w-4" aria-hidden="true" />
        Logout
      </button>
    </div>
  );
}

export function AppShell() {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);

  useEffect(() => {
    if (!drawerOpen) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setDrawerOpen(false);
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [drawerOpen]);

  const handleLogout = () => {
    setDrawerOpen(false);
    logout();
    navigate('/login', { replace: true });
  };

  return (
    <div className="min-h-screen bg-surface">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 flex-col border-r border-warm-200 bg-surface-card px-5 py-6 lg:flex">
        <BrandMark />
        <div className="mt-8 flex-1">
          <NavItems />
        </div>
        <UserCard onLogout={handleLogout} />
      </aside>

      <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-warm-200 bg-surface-card/90 px-4 backdrop-blur lg:hidden">
        <BrandMark />
        <button
          type="button"
          onClick={() => setDrawerOpen(true)}
          aria-label="Open navigation menu"
          aria-expanded={drawerOpen}
          className="rounded-xl p-2 text-stone-600 transition-colors hover:bg-warm-100 hover:text-stone-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          <Menu className="h-6 w-6" aria-hidden="true" />
        </button>
      </header>

      {drawerOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div
            className="absolute inset-0 bg-stone-950/40 backdrop-blur-sm"
            onClick={() => setDrawerOpen(false)}
            aria-hidden="true"
          />
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Navigation menu"
            className="absolute inset-y-0 left-0 flex w-80 max-w-[85vw] flex-col bg-surface-card px-5 py-6 shadow-2xl"
          >
            <div className="flex items-center justify-between">
              <BrandMark />
              <button
                type="button"
                onClick={() => setDrawerOpen(false)}
                aria-label="Close navigation menu"
                className="rounded-xl p-2 text-stone-500 transition-colors hover:bg-warm-100 hover:text-stone-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
              >
                <X className="h-5 w-5" aria-hidden="true" />
              </button>
            </div>
            <div className="mt-8 flex-1">
              <NavItems onNavigate={() => setDrawerOpen(false)} />
            </div>
            <UserCard
              onLogout={handleLogout}
              onNavigate={() => setDrawerOpen(false)}
            />
          </div>
        </div>
      )}

      <main className="lg:pl-72">
        <div className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 lg:px-10 lg:py-10">
          <Outlet />
        </div>
      </main>

      <Toaster />
    </div>
  );
}
