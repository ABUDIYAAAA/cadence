'use client';

import React, { useState, useMemo, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import {
	Award,
	Plus,
	RefreshCw,
	Search,
	Filter,
	CheckCircle2,
	Clock,
	Layers,
	DoorOpen,
	UserCheck,
	CheckSquare,
} from 'lucide-react';
import { useAuth } from '@/providers/auth-provider';
import { useToast } from '@/providers/toast-provider';
import {
	useJudgingRounds,
	useActivateRound,
	useEndRound,
	useDeleteRound,
	useCreateRound,
	useCreatePanel,
	useConfigureSlots,
} from '@/hooks/use-judging';
import {
	RoundSummary,
	CreateRoundRequest,
	CreatePanelRequest,
	ConfigureSlotsRequest,
	SlotAssignment,
} from '@/types/judging';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { apiErrorMessage } from '@/lib/utils';
import { SkeletonCard } from '@/components/ui/skeleton';
import {
	RoundTable,
	RoundCreateDialog,
	RoundPanelCreateDialog,
	RoundSlotsConfigDialog,
	RoundTeamReassignDialog,
	RoundDetailDialog,
	RoundLeaderboardView,
	RoundActivateDialog,
	RoundConfirmDialog,
	VolunteerScheduleView,
	ParticipantJudgingStatusView,
	JudgeEvaluationView,
} from '@/components/judging';

type JudgingTab = 'rounds' | 'volunteer_monitor' | 'leaderboard';

export default function JudgingPage() {
	const router = useRouter();
	const toast = useToast();
	const {
		user,
		isAdmin,
		isJudge,
		isVolunteer,
		isParticipant,
		isLoading: authLoading,
	} = useAuth();

	// Admins manage rounds and monitor; judges only see their scoring screen (handled below).
	const [activeTab, setActiveTab] = useState<JudgingTab>('rounds');

	// Queries & Mutations
	const {
		data: rounds = [],
		isLoading: roundsLoading,
		isFetching,
		refetch,
	} = useJudgingRounds();

	const activateRoundMutation = useActivateRound();
	const endRoundMutation = useEndRound();
	const deleteRoundMutation = useDeleteRound();
	const createRoundMutation = useCreateRound();
	const createPanelMutation = useCreatePanel();
	const configureSlotsMutation = useConfigureSlots();

	// Dialog State
	const [isCreateOpen, setIsCreateOpen] = useState(false);
	const [isCreatingWorkflow, setIsCreatingWorkflow] = useState(false);
	const [selectedRoundForDetails, setSelectedRoundForDetails] =
		useState<RoundSummary | null>(null);
	const [selectedRoundForPanel, setSelectedRoundForPanel] =
		useState<RoundSummary | null>(null);
	const [selectedRoundForSlots, setSelectedRoundForSlots] =
		useState<RoundSummary | null>(null);
	const [selectedRoundForReassign, setSelectedRoundForReassign] =
		useState<RoundSummary | null>(null);
	const [roundToEnd, setRoundToEnd] = useState<RoundSummary | null>(null);
	const [roundToDelete, setRoundToDelete] = useState<RoundSummary | null>(null);
	const [selectedRoundForActivation, setSelectedRoundForActivation] =
		useState<RoundSummary | null>(null);

	// Search & Filters
	const [searchQuery, setSearchQuery] = useState('');
	const [statusFilter, setStatusFilter] = useState<string>('all');

	// Filtering rounds
	const filteredRounds = useMemo(() => {
		return (rounds || []).filter((round) => {
			const matchesSearch =
				searchQuery.trim() === '' ||
				round.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
				round.id.toLowerCase().includes(searchQuery.toLowerCase());

			const matchesStatus =
				statusFilter === 'all' || round.status === statusFilter;

			return matchesSearch && matchesStatus;
		});
	}, [rounds, searchQuery, statusFilter]);

	// Metric stats
	const stats = useMemo(() => {
		const list = rounds || [];
		const total = list.length;
		const active = list.filter((r) => r.status === 'active').length;
		const completed = list.filter((r) => r.status === 'completed').length;
		const draft = list.filter((r) => r.status === 'draft').length;
		const totalPanels = list.reduce(
			(acc, r) => acc + (r.panel_count || 0),
			0,
		);
		const totalSlots = list.reduce(
			(acc, r) => acc + (r.slot_count || 0),
			0,
		);

		return { total, active, completed, draft, totalPanels, totalSlots };
	}, [rounds]);

	// Auth Guards
	useEffect(() => {
		if (!authLoading && !user) {
			router.replace('/login');
		}
	}, [authLoading, user, router]);

	if (authLoading || !user) {
		return (
			<div className='font-mono space-y-4'>
				<SkeletonCard />
				<SkeletonCard />
			</div>
		);
	}

	// 1. Participant dedicated judging view
	if (isParticipant && !isAdmin && !isJudge && !isVolunteer) {
		return (
			<div className='font-sans'>
				<ParticipantJudgingStatusView />
			</div>
		);
	}

	// 2. Volunteer dedicated room & schedule coordination view
	if (isVolunteer && !isAdmin && !isJudge) {
		return (
			<div className='font-mono'>
				<VolunteerScheduleView />
			</div>
		);
	}

	// 3. Admin & Judge View Handlers
	const handleOpenActivate = (roundId: string) => {
		if (!isAdmin) {
			toast.error('Only administrators can activate judging rounds.');
			return;
		}
		const r = rounds.find((item) => item.id === roundId);
		if (r) {
			setSelectedRoundForActivation(r);
		}
	};

	const handleConfirmActivation = async () => {
		if (!selectedRoundForActivation) return;
		try {
			await activateRoundMutation.mutateAsync(
				selectedRoundForActivation.id,
			);
			toast.success(
				`Round "${selectedRoundForActivation.name}" has started.`,
			);
			setSelectedRoundForActivation(null);
			refetch();
		} catch (err: unknown) {
			const message =
				apiErrorMessage(err, 'Failed to activate round');
			toast.error(message);
		}
	};

	const handleConfirmEnd = async () => {
		if (!roundToEnd) return;
		try {
			await endRoundMutation.mutateAsync(roundToEnd.id);
			toast.success(`Round "${roundToEnd.name}" has ended.`);
			setRoundToEnd(null);
		} catch (err: unknown) {
			toast.error(apiErrorMessage(err, 'Failed to end round'));
		}
	};

	const handleConfirmDelete = async () => {
		if (!roundToDelete) return;
		try {
			await deleteRoundMutation.mutateAsync(roundToDelete.id);
			toast.success(`Round "${roundToDelete.name}" was deleted.`);
			setRoundToDelete(null);
		} catch (err: unknown) {
			toast.error(apiErrorMessage(err, 'Failed to delete round'));
		}
	};

	const handleFullWorkflowSubmit = async (params: {
		round: CreateRoundRequest;
		panels: Array<{
			name: string;
			room: string;
			judge_ids: string[];
			slots: SlotAssignment[];
		}>;
	}) => {
		setIsCreatingWorkflow(true);
		try {
			// 1. Create Round
			const createdRound = await createRoundMutation.mutateAsync(
				params.round,
			);
			const roundId = createdRound.round_id || createdRound.id;

			if (!roundId) {
				throw new Error(
					'Round created but server did not return a round ID',
				);
			}

			// 2. Create Panels and configure Slots if any
			if (params.panels && params.panels.length > 0) {
				for (const p of params.panels) {
					const panelRes = await createPanelMutation.mutateAsync({
						round_id: roundId,
						name: p.name,
						room: p.room,
						judge_ids: p.judge_ids,
					});

					const panelId = panelRes.panel_id || panelRes.id;

					if (panelId && p.slots && p.slots.length > 0) {
						await configureSlotsMutation.mutateAsync({
							panelId,
							payload: {
								slots: p.slots,
							},
						});
					}
				}
			}

			const successMessage =
				params.panels && params.panels.length > 0
					? `Judging round "${params.round.name}" created with ${params.panels.length} room(s) and team slots!`
					: `Judging round "${params.round.name}" created successfully!`;
			toast.success(successMessage);
			setIsCreateOpen(false);
			refetch();
		} catch (err: unknown) {
			const message =
				apiErrorMessage(err, 'Failed to create round workflow');
			toast.error(message);
			throw err;
		} finally {
			setIsCreatingWorkflow(false);
		}
	};

	const handlePanelCreate = async (payload: CreatePanelRequest) => {
		try {
			await createPanelMutation.mutateAsync(payload);
			toast.success(`Panel "${payload.name}" added successfully.`);
			setSelectedRoundForPanel(null);
			refetch();
		} catch (err: unknown) {
			const message =
				apiErrorMessage(err, 'Failed to create panel');
			toast.error(message);
		}
	};

	const handleSlotsConfigure = async (
		panelId: string,
		payload: ConfigureSlotsRequest,
	) => {
		try {
			await configureSlotsMutation.mutateAsync({ panelId, payload });
			toast.success(
				`Configured ${payload.slots.length} team slot(s) for the room.`,
			);
			setSelectedRoundForSlots(null);
			refetch();
		} catch (err: unknown) {
			const message =
				apiErrorMessage(err, 'Failed to configure slots');
			toast.error(message);
		}
	};

	// 3. Judges only score: no admin tooling here
	if (isJudge && !isAdmin) {
		return (
			<div className='font-sans'>
				<JudgeEvaluationView />
			</div>
		);
	}

	return (
		<div className='space-y-6 font-sans'>
			{/* View Switcher Tabs (admin) */}
			<div className='flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4'>
				<div>
					<h1 className='text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]'>
						Judging
					</h1>
					<p className='text-sm text-[var(--ink-secondary)] mt-1'>
						Manage scoring rounds, room assignments, and submissions.
					</p>
				</div>

				<div className='flex items-center gap-2 flex-wrap'>
					<div className='inline-flex rounded-lg border border-[var(--card-line)] p-1 bg-[var(--card-solid-subtle)] gap-1 flex-wrap'>
						{/* Rounds Registry / Configuration */}
						<button
							onClick={() => setActiveTab('rounds')}
							className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
								activeTab === 'rounds'
									? 'bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold'
									: 'text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium'
							}`}>
							Rounds config
						</button>

						{/* Room Staging Monitor */}
						<button
							onClick={() => setActiveTab('volunteer_monitor')}
							className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
								activeTab === 'volunteer_monitor'
									? 'bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold'
									: 'text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium'
							}`}>
							Room staging
						</button>

						<button
							onClick={() => setActiveTab('leaderboard')}
							className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
								activeTab === 'leaderboard'
									? 'bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold'
									: 'text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium'
							}`}>
							Leaderboard
						</button>

					</div>
				</div>
			</div>

			{/* TAB 2: VOLUNTEER STAGING SCHEDULE MONITOR */}
			{activeTab === 'volunteer_monitor' && <VolunteerScheduleView />}

			{/* LEADERBOARD (admin) */}
			{activeTab === 'leaderboard' && <RoundLeaderboardView />}

			{/* TAB 4: ROUNDS MANAGEMENT & CONFIGURATION */}
			{activeTab === 'rounds' && (
				<>
					{/* Action Toolbar */}
					<div className='flex items-center justify-between gap-4'>
						<div className='text-sm font-semibold text-[var(--ink-primary)]'>
							Rounds directory
						</div>

						<div className='flex items-center gap-2 flex-wrap'>
							<Button
								variant='outline'
								size='sm'
								onClick={() => refetch()}
								disabled={isFetching}
								className='flex items-center gap-1.5 text-xs h-9'>
								<RefreshCw
									className={`h-3.5 w-3.5 ${isFetching ? 'animate-spin' : ''}`}
								/>
								Refresh
							</Button>

							{isAdmin && (
								<Button
									size='sm'
									variant='primary'
									onClick={() => setIsCreateOpen(true)}
									className='flex items-center gap-1.5 text-xs h-9 px-4 font-semibold'>
									<Plus className='h-4 w-4' />
									Create round
								</Button>
							)}
						</div>
					</div>

					{/* Metrics Banner */}
					<div className='grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3'>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Total rounds
							</div>
							<div className='text-2xl font-bold font-mono text-[var(--ink-primary)] mt-1'>
								{stats.total}
							</div>
						</div>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Active
							</div>
							<div className='text-2xl font-bold font-mono text-[var(--color-blue)] mt-1'>
								{stats.active}
							</div>
						</div>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Draft
							</div>
							<div className='text-2xl font-bold font-mono text-[var(--ink-secondary)] mt-1'>
								{stats.draft}
							</div>
						</div>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Completed
							</div>
							<div className='text-2xl font-bold font-mono text-[#2EB88A] mt-1'>
								{stats.completed}
							</div>
						</div>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Total panels
							</div>
							<div className='text-2xl font-bold font-mono text-[var(--ink-primary)] mt-1'>
								{stats.totalPanels}
							</div>
						</div>
						<div className='rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5'>
							<div className='text-xs text-[var(--ink-secondary)] font-medium'>
								Total slots
							</div>
							<div className='text-2xl font-bold font-mono text-[var(--ink-primary)] mt-1'>
								{stats.totalSlots}
							</div>
						</div>
					</div>

					{/* Primary Surface: Search/Filters + Table */}
					<div className='bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4'>
						<div className='flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3'>
							<div className='relative flex-1'>
								<Search className='absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--ink-tertiary)]' />
								<Input
									placeholder='Search rounds by name or ID...'
									value={searchQuery}
									onChange={(e) => setSearchQuery(e.target.value)}
									className='pl-9 text-xs'
								/>
							</div>

							<div className='flex items-center gap-2'>
								<span className='text-xs font-medium text-[var(--ink-secondary)] shrink-0'>
									Status:
								</span>
								<div className='w-44'>
									<Select
										value={statusFilter}
										onChange={(e) =>
											setStatusFilter(e.target.value)
										}
										options={[
											{ value: 'all', label: 'All statuses' },
											{ value: 'draft', label: 'Draft' },
											{ value: 'active', label: 'Active' },
											{
												value: 'completed',
												label: 'Completed',
											},
										]}
										className='text-xs'
									/>
								</div>
							</div>
						</div>

						{/* Rounds Listing Table */}
						<RoundTable
							rounds={filteredRounds}
							isLoading={roundsLoading}
							onView={(round: RoundSummary) =>
								setSelectedRoundForDetails(round)
							}
							onAddPanel={(round: RoundSummary) =>
								setSelectedRoundForPanel(round)
							}
							onConfigureSlots={(round: RoundSummary) =>
								setSelectedRoundForSlots(round)
							}
							onReassignTeams={(round: RoundSummary) =>
								setSelectedRoundForReassign(round)
							}
							onEnd={(round: RoundSummary) => setRoundToEnd(round)}
							onDelete={(round: RoundSummary) => setRoundToDelete(round)}
							onActivate={(roundId: string) =>
								handleOpenActivate(roundId)
							}
							isActivatingId={
								activateRoundMutation.isPending
									? selectedRoundForActivation?.id
									: null
							}
							isAdmin={isAdmin}
						/>
					</div>
				</>
			)}

			{/* Modals & Dialogs */}
			{/* 1. Main Round Creation Wizard (Rounds -> Panels -> Teams -> Slots) */}
			<RoundCreateDialog
				open={isCreateOpen}
				onClose={() => setIsCreateOpen(false)}
				onSubmitFullWorkflow={handleFullWorkflowSubmit}
				isLoading={isCreatingWorkflow}
			/>

			{/* 2. Quick Panel Addition Modal */}
			<RoundPanelCreateDialog
				open={Boolean(selectedRoundForPanel)}
				onClose={() => setSelectedRoundForPanel(null)}
				round={selectedRoundForPanel}
				onSubmit={handlePanelCreate}
				isLoading={createPanelMutation.isPending}
			/>

			{/* 3. Quick Slot Configuration Modal */}
			<RoundSlotsConfigDialog
				open={Boolean(selectedRoundForSlots)}
				onClose={() => setSelectedRoundForSlots(null)}
				round={selectedRoundForSlots}
				onSubmit={handleSlotsConfigure}
				isLoading={configureSlotsMutation.isPending}
			/>

			{/* Reassign teams between rooms (draft rounds) */}
			<RoundTeamReassignDialog
				open={Boolean(selectedRoundForReassign)}
				onClose={() => setSelectedRoundForReassign(null)}
				round={selectedRoundForReassign}
			/>

			{/* 4. Round Criteria & Details Modal */}
			<RoundDetailDialog
				open={Boolean(selectedRoundForDetails)}
				onClose={() => setSelectedRoundForDetails(null)}
				round={selectedRoundForDetails}
				onActivate={(roundId: string) => handleOpenActivate(roundId)}
				isActivating={activateRoundMutation.isPending}
			/>

			<RoundConfirmDialog
				mode='end'
				open={Boolean(roundToEnd)}
				onClose={() => setRoundToEnd(null)}
				round={roundToEnd}
				onConfirm={handleConfirmEnd}
				isLoading={endRoundMutation.isPending}
			/>

			<RoundConfirmDialog
				mode='delete'
				open={Boolean(roundToDelete)}
				onClose={() => setRoundToDelete(null)}
				round={roundToDelete}
				onConfirm={handleConfirmDelete}
				isLoading={deleteRoundMutation.isPending}
			/>

			{/* 5. Custom In-App Activation Confirmation Dialog (NO NATIVE ALERTS) */}
			<RoundActivateDialog
				open={Boolean(selectedRoundForActivation)}
				onClose={() => setSelectedRoundForActivation(null)}
				round={selectedRoundForActivation}
				onConfirm={handleConfirmActivation}
				isLoading={activateRoundMutation.isPending}
			/>
		</div>
	);
}
