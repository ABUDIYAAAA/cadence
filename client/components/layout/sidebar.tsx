'use client';

import React, { useState, useEffect, useRef } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
	Users,
	LayoutDashboard,
	Menu,
	X,
	Layers,
	Award,
	Ticket,
	FileText,
	Bell,
	Activity,
	ArrowUpRight,
	Calendar,
	ChevronUp,
	LogOut,
	Settings,
} from 'lucide-react';
import { useAuth } from '@/providers/auth-provider';
import { useTenant } from '@/providers/tenant-provider';
import { useToast } from '@/providers/toast-provider';
import { useNotificationsContext } from '@/providers/notification-provider';
import { getDashboardRoute } from '@/lib/auth-routes';
import { UserRole } from '@/types/auth';
import { cn } from '@/lib/utils';

interface NavItem {
	label: string;
	href: string;
	icon: React.ComponentType<{ className?: string }>;
	roles?: UserRole[];
	external?: boolean;
}

// Fixed geometry so one indicator can glide between rows with a single transform.
const ITEM_H = 44;
const ITEM_GAP = 4;
const EASE_DRAWER = 'ease-[cubic-bezier(0.32,0.72,0,1)]';

interface NavListProps {
	items: NavItem[];
	activeIndex: number;
	/** Labels always visible (mobile drawer). Otherwise revealed by rail hover/focus. */
	pinned?: boolean;
	onNavigate?: () => void;
}

function NavList({ items, activeIndex, pinned, onNavigate }: NavListProps) {
	const labelCls = cn(
		'whitespace-nowrap text-[13px] transition-opacity duration-200 motion-reduce:transition-none',
		pinned
			? 'opacity-100'
			: 'opacity-0 group-hover/rail:opacity-100 group-has-[:focus-visible]/rail:opacity-100',
	);

	return (
		<nav
			className='relative flex flex-col'
			style={{ gap: ITEM_GAP }}
			aria-label='Primary'>
			{/* Single gliding indicator */}
			<span
				aria-hidden='true'
				className={cn(
					'absolute left-0 top-0 w-full rounded-[14px] bg-[var(--color-dark)] shadow-[0_6px_16px_-6px_rgba(10,14,24,0.45)]',
					'transition-[transform,opacity] duration-300 motion-reduce:transition-none',
					EASE_DRAWER,
					activeIndex < 0 ? 'opacity-0' : 'opacity-100',
				)}
				style={{
					height: ITEM_H,
					transform: `translateY(${Math.max(activeIndex, 0) * (ITEM_H + ITEM_GAP)}px)`,
				}}
			/>

			{items.map((item, i) => {
				const active = i === activeIndex;
				const Icon = item.icon;
				const base = cn(
					'relative z-10 flex items-center rounded-[14px] outline-none',
					'focus-visible:ring-2 focus-visible:ring-[var(--color-blue)] focus-visible:ring-offset-0',
					'transition-[color,background-color,transform] duration-150 active:scale-[0.97]',
					active
						? 'text-[var(--ink-inverse)]'
						: 'text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]',
				);

				const inner = (
					<>
						<span
							className='grid place-items-center shrink-0'
							style={{ width: ITEM_H, height: ITEM_H }}>
							<Icon className='h-[18px] w-[18px]' />
						</span>
						<span
							className={cn(
								labelCls,
								active ? 'font-semibold' : 'font-medium',
							)}>
							{item.label}
						</span>
						{item.external && (
							<ArrowUpRight
								className={cn(
									'ml-auto mr-3 h-3.5 w-3.5 shrink-0',
									labelCls,
								)}
							/>
						)}
					</>
				);

				return item.external ? (
					<a
						key={item.href}
						href={item.href}
						target='_blank'
						rel='noopener noreferrer'
						className={base}
						style={{ height: ITEM_H }}>
						{inner}
					</a>
				) : (
					<Link
						key={item.href}
						href={item.href}
						onClick={onNavigate}
						aria-current={active ? 'page' : undefined}
						className={base}
						style={{ height: ITEM_H }}>
						{inner}
					</Link>
				);
			})}
		</nav>
	);
}

function UserChip({
	name,
	role,
	avatarUrl,
	pinned,
	onOpenChange,
	onNavigate,
}: {
	name: string;
	role: string;
	avatarUrl?: string | null;
	pinned?: boolean;
	onOpenChange?: (open: boolean) => void;
	onNavigate?: () => void;
}) {
	const { isAdmin, logout } = useAuth();
	const { eventSlug } = useTenant();
	const { isSupported, isSubscribed, enablePush, disablePush } =
		useNotificationsContext();
	const toast = useToast();

	const [avatarFailed, setAvatarFailed] = useState(false);
	const [menuOpen, setMenuOpen] = useState(false);
	const [isLoggingOut, setIsLoggingOut] = useState(false);
	const containerRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		setAvatarFailed(false);
	}, [avatarUrl]);

	useEffect(() => {
		onOpenChange?.(menuOpen);
	}, [menuOpen, onOpenChange]);

	useEffect(() => {
		if (!menuOpen) return;
		const handleClickOutside = (event: MouseEvent) => {
			if (
				containerRef.current &&
				!containerRef.current.contains(event.target as Node)
			) {
				setMenuOpen(false);
			}
		};
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Escape') {
				setMenuOpen(false);
			}
		};
		document.addEventListener('mousedown', handleClickOutside);
		document.addEventListener('keydown', handleKeyDown);
		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
			document.removeEventListener('keydown', handleKeyDown);
		};
	}, [menuOpen]);

	const handleLogout = async () => {
		setIsLoggingOut(true);
		try {
			await logout();
			setMenuOpen(false);
			onNavigate?.();
			toast.info('You have been signed out.', 'Session Ended');
		} finally {
			setIsLoggingOut(false);
		}
	};

	const initials = name
		.split(' ')
		.map((p) => p[0])
		.filter(Boolean)
		.slice(0, 2)
		.join('')
		.toUpperCase();

	const showAvatarImage = Boolean(avatarUrl && !avatarFailed);
	const showEventSettings = Boolean(eventSlug && isAdmin);
	const showPushAlerts = Boolean(isSupported && eventSlug);

	return (
		<div ref={containerRef} className='relative'>
			{menuOpen && (
				<div className='absolute bottom-full left-0 right-0 mb-2 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] z-50 p-1.5 text-[var(--ink-primary)] font-sans'>
					<div className='space-y-0.5'>
						<Link
							href='/events'
							onClick={() => {
								setMenuOpen(false);
								onNavigate?.();
							}}
							className='flex items-center gap-2 px-2.5 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)]'>
							<Calendar className='h-3.5 w-3.5 text-[var(--ink-secondary)] shrink-0' />
							<span className='truncate'>Switch event</span>
						</Link>
						{showEventSettings && (
							<button
								type='button'
								onClick={() => {
									setMenuOpen(false);
									onNavigate?.();
									window.dispatchEvent(
										new CustomEvent('cadence:open-event-settings'),
									);
								}}
								className='w-full flex items-center gap-2 px-2.5 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)] cursor-pointer'>
								<Settings className='h-3.5 w-3.5 text-[var(--ink-secondary)] shrink-0' />
								<span className='truncate'>Event settings</span>
							</button>
						)}
					</div>

					{showPushAlerts && (
						<div className='mt-1 pt-1.5 pb-1 px-2.5 border-t border-[var(--card-line)] flex items-center justify-between gap-2 text-xs'>
							<span className='flex items-center gap-1.5 text-[var(--ink-secondary)] min-w-0'>
								<Bell className='h-3.5 w-3.5 shrink-0' />
								<span className='truncate'>Push alerts</span>
							</span>
							<button
								type='button'
								onClick={() =>
									isSubscribed ? disablePush() : enablePush()
								}
								className='text-[11px] font-medium rounded-[6px] px-2 py-0.5 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-primary)] hover:bg-[var(--card-solid)] transition-colors cursor-pointer shrink-0'>
								{isSubscribed ? 'Active' : 'Enable'}
							</button>
						</div>
					)}

					<div className='pt-1 border-t border-[var(--card-line)] mt-1'>
						<button
							type='button'
							onClick={handleLogout}
							disabled={isLoggingOut}
							className='w-full flex items-center gap-2 px-2.5 py-2 text-xs rounded-[6px] text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer font-medium disabled:opacity-50'>
							<LogOut className='h-3.5 w-3.5 shrink-0' />
							<span className='truncate'>
								{isLoggingOut ? 'Signing out...' : 'Log out'}
							</span>
						</button>
					</div>
				</div>
			)}

			<button
				type='button'
				onClick={() => setMenuOpen((prev) => !prev)}
				aria-expanded={menuOpen}
				aria-label='Account menu'
				className='w-full flex items-center rounded-[14px] hover:bg-[var(--card-solid-subtle)] transition-colors cursor-pointer text-left'
				style={{ height: ITEM_H }}>
				<span
					className='grid place-items-center shrink-0'
					style={{ width: ITEM_H, height: ITEM_H }}>
					<span className='grid h-8 w-8 place-items-center overflow-hidden rounded-full bg-[var(--color-blue-dim)] text-[11px] font-semibold text-[var(--color-blue)] ring-1 ring-[var(--color-blue-border)]'>
						{showAvatarImage ? (
							<img
								src={avatarUrl!}
								alt={name || 'User avatar'}
								referrerPolicy='no-referrer'
								onError={() => setAvatarFailed(true)}
								className='h-full w-full object-cover'
							/>
						) : (
							initials || '?'
						)}
					</span>
				</span>
				<span
					className={cn(
						'flex-1 min-w-0 flex items-center justify-between gap-2 pr-2.5 leading-tight transition-opacity duration-200 motion-reduce:transition-none',
						pinned || menuOpen
							? 'opacity-100'
							: 'opacity-0 group-hover/rail:opacity-100 group-has-[:focus-visible]/rail:opacity-100',
					)}>
					<span className='min-w-0'>
						<span className='block truncate text-[13px] font-semibold text-[var(--ink-primary)]'>
							{name}
						</span>
						<span className='block truncate text-[11px] capitalize text-[var(--ink-secondary)]'>
							{role}
						</span>
					</span>
					<ChevronUp
						className={cn(
							'h-3.5 w-3.5 text-[var(--ink-tertiary)] shrink-0 transition-transform duration-150',
							menuOpen ? 'rotate-180' : '',
						)}
					/>
				</span>
			</button>
		</div>
	);
}

export function Sidebar() {
	const pathname = usePathname();
	const { user, role, effectiveRole: authEffectiveRole, isAdmin, isDev } = useAuth();
	const { eventSlug } = useTenant();
	const [isOpenMobile, setIsOpenMobile] = useState(false);
	const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);

	// Listen for toggle events from the Navbar mobile button
	useEffect(() => {
		const handleToggle = () => setIsOpenMobile((prev) => !prev);
		window.addEventListener('cadence:toggle-sidebar', handleToggle);
		return () =>
			window.removeEventListener('cadence:toggle-sidebar', handleToggle);
	}, []);

	useEffect(() => {
		if (!isOpenMobile) return;
		const onKey = (e: KeyboardEvent) =>
			e.key === 'Escape' && setIsOpenMobile(false);
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	}, [isOpenMobile]);

	const isPlatformDevMode = isDev && !eventSlug;
	const effectiveRole = isPlatformDevMode
		? 'dev'
		: (authEffectiveRole ?? role ?? user?.role);
	const dashboardHref = isPlatformDevMode
		? '/dashboard/dev'
		: effectiveRole
		? getDashboardRoute(effectiveRole)
		: '/dashboard';

	const devNavItems: NavItem[] = [
		{ label: 'Overview', href: '/dashboard/dev', icon: LayoutDashboard },
		{ label: 'Global users', href: '/dashboard/users', icon: Users },
		{ label: 'Events', href: '/events', icon: Calendar },
		{ label: 'Audit log', href: '/dashboard/audit', icon: FileText },
		{
			label: 'Metrics',
			href: 'https://grafana.cadence.devclubxnst.com/',
			icon: Activity,
			external: true,
		},
	];

	const allNavItems: NavItem[] = [
		{ label: 'Overview', href: dashboardHref, icon: LayoutDashboard },
		{
			label: 'Users',
			href: '/dashboard/users',
			icon: Users,
			roles: ['owner', 'admin', 'volunteer'],
		},
		{
			label: 'Teams',
			href: '/dashboard/teams',
			icon: Layers,
			roles: ['owner', 'admin', 'volunteer', 'judge'],
		},
		{
			label: 'Broadcasts',
			href: '/dashboard/admin/notifications',
			icon: Bell,
			roles: ['owner', 'admin'],
		},
		{
			label: 'Judging',
			href: '/dashboard/judging',
			icon: Award,
			roles: ['owner', 'admin', 'judge', 'volunteer', 'participant'],
		},
		{
			label: 'Tickets',
			href: '/dashboard/tickets',
			icon: Ticket,
			roles: ['owner', 'admin', 'volunteer', 'participant'],
		},
		{
			label: 'Audit log',
			href: '/dashboard/audit',
			icon: FileText,
			roles: ['owner', 'admin'],
		},
	];

	const visibleNavItems = isPlatformDevMode
		? devNavItems
		: allNavItems.filter(
				(item) =>
					!item.roles ||
					isDev ||
					(effectiveRole && item.roles.includes(effectiveRole)) ||
					(isAdmin && item.roles.includes('admin')),
			);

	const isActive = (item: NavItem) => {
		if (item.external) return false;
		if (item.href === '/dashboard' || item.href === dashboardHref)
			return pathname === item.href;
		return pathname.startsWith(item.href);
	};
	const activeIndex = visibleNavItems.findIndex(isActive);

	return (
		<>
			{/* Mobile Floating Toggle Button */}
			<div className='md:hidden fixed bottom-5 right-5 z-50'>
				<button
					onClick={() => setIsOpenMobile(!isOpenMobile)}
					aria-label='Toggle navigation menu'
					aria-expanded={isOpenMobile}
					className='h-12 w-12 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-lg flex items-center justify-center transition-transform duration-150 active:scale-[0.95] cursor-pointer'>
					{isOpenMobile ? (
						<X className='h-5 w-5' />
					) : (
						<Menu className='h-5 w-5' />
					)}
				</button>
			</div>

			{/* Mobile Drawer */}
			<div
				className={cn(
					'fixed inset-0 z-40 md:hidden',
					isOpenMobile
						? 'pointer-events-auto visible'
						: 'pointer-events-none invisible delay-300',
				)}>
				<div
					className={cn(
						'fixed inset-0 bg-black/40 backdrop-blur-xs transition-opacity duration-200 ease-out',
						isOpenMobile ? 'opacity-100' : 'opacity-0',
					)}
					onClick={() => setIsOpenMobile(false)}
					aria-hidden='true'
				/>
				<aside
					className={cn(
						'fixed inset-y-0 left-0 w-64 bg-[var(--card-solid)] p-3 pt-5 shadow-2xl flex flex-col justify-between font-sans',
						'transition-transform duration-300 motion-reduce:transition-none',
						EASE_DRAWER,
						isOpenMobile ? 'translate-x-0' : '-translate-x-full',
					)}>
					<NavList
						items={visibleNavItems}
						activeIndex={activeIndex}
						pinned
						onNavigate={() => setIsOpenMobile(false)}
					/>
					{user && (
						<div className='pt-3 border-t border-[var(--card-line)]'>
							<UserChip
								name={user.name}
								role={user.role}
								avatarUrl={user.avatar_url}
								pinned
								onNavigate={() => setIsOpenMobile(false)}
							/>
						</div>
					)}
				</aside>
			</div>

			{/* Desktop rail: reserves 72px, expands over content on hover/focus (no layout shift) */}
			<div className='hidden md:block relative shrink-0 w-[72px] sticky top-14 h-[calc(100vh-3.5rem)] z-30'>
				<aside
					className={cn(
						'group/rail absolute inset-y-0 left-0 flex flex-col justify-between overflow-hidden font-sans',
						'w-[72px] hover:w-[232px] has-[:focus-visible]:w-[232px] p-3',
						'bg-[var(--card-solid)] border-r border-[var(--card-line)]',
						'transition-[width,box-shadow] duration-300 motion-reduce:transition-none',
						EASE_DRAWER,
						'hover:shadow-[12px_0_32px_-16px_rgba(10,14,24,0.25)] has-[:focus-visible]:shadow-[12px_0_32px_-16px_rgba(10,14,24,0.25)]',
						isUserMenuOpen && 'w-[232px] shadow-[12px_0_32px_-16px_rgba(10,14,24,0.25)]',
					)}>
					<NavList
						items={visibleNavItems}
						activeIndex={activeIndex}
						pinned={isUserMenuOpen}
					/>
					{user && (
						<div className='pt-3 border-t border-[var(--card-line)]'>
							<UserChip
								name={user.name}
								role={user.role}
								avatarUrl={user.avatar_url}
								pinned={isUserMenuOpen}
								onOpenChange={setIsUserMenuOpen}
							/>
						</div>
					)}
				</aside>
			</div>
		</>
	);
}
