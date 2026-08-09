import { useState } from 'react';
import { CountdownTimer } from './CountdownTimer';
import { PhaseCard } from './PhaseCard';
import { CreatePhaseModal } from './CreatePhaseModal';
import type { DraftPostData } from './CreatePhaseModal';
import { EditPhaseModal } from './EditPhaseModal';
import { usePhaseManagement } from '../hooks/usePhaseManagement';
import { apiClient } from '../lib/api';
import { Button } from './ui';
import { PHASE_TYPE_LABELS } from '../types/phases';
import type { GamePhase, CreatePhaseRequest } from '../types/phases';
import { localDateTimeToUTC } from '../utils/timezone';
import { useToast } from '../contexts/ToastContext';
import { useLootTableManagement } from '@/hooks/useLootTablemanagement';
import type { CreateLootTableRequest, LootTable } from '@/types/games';
import { LootTableForm } from './loot-tables/LootTableForm';
import { useUrlParam } from '@/hooks/useUrlParam';
import { Link } from 'react-router-dom';
import { PencilIcon, TrashIcon, XMarkIcon } from '@heroicons/react/24/outline';

interface LootTablesProps {
  gameId: number;
  className?: string;
}


const lootTableParamOptions = {
  deserialize: (s: string) => parseInt(s, 10) || null,
  serialize: (v: number | null) => (v === null || v === undefined ? '' : String(v)),
} as const;

const newLootTableParamOptions = {
  deserialize: (s: string) => s === 'true',
  serialize: (v: boolean | null) => (v === null || v === undefined ? '' : String(v)),
} as const;

export function LootTablesView({ gameId, className = '' }: LootTablesProps) {
  const { showWarning } = useToast();
  const [editingLootTable, setEditingLootTable] = useState<LootTable | null>(null);
  
  const [selectedLootTableId, setSelectedLootTableId] = useUrlParam<number | null>(
    'table',
    null,
    { ...lootTableParamOptions, replace: false }
  );
  
  const [newLootTable, setNewLootTable] = useUrlParam<boolean | null>(
    'new',
    null,
    { ...newLootTableParamOptions, replace: false }
  );

  const {
    lootTables,
    isLoading,
    createLootTableMutation,
    deleteLootTableMutation
  } = useLootTableManagement(gameId);

  const selectedTable = lootTables.find(t => t.id === selectedLootTableId);

  const deleteLootTable = async (lootTableId: number) => {
    const confirmed = window.confirm('Are you sure you want to delete this loot table? This action cannot be undone.');
    if (!confirmed) {
      return;
    }
    await deleteLootTableMutation.mutateAsync(lootTableId);
  }

  if (isLoading) {
    return (
      <div className={`surface-base rounded-lg border border-theme-default p-6 ${className}`}>
        <div className="animate-pulse">
          <div className="h-6 surface-sunken rounded mb-4 w-1/3"></div>
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-16 surface-sunken rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if ((!!selectedLootTableId && !!selectedTable) || newLootTable) {
    return <div>
        <Button
          variant="ghost"
          onClick={() => {
            if (!!selectedLootTableId) {
              setSelectedLootTableId(null); 
            }
            if (!!newLootTable) {
              setNewLootTable(null); 
            }
          }}
          className="mb-4 flex items-center text-interactive-primary hover:text-interactive-primary-hover"
        >
          <svg className="w-5 h-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to Loot Tables
        </Button>
      <LootTableForm
        onClose={() => {setSelectedLootTableId(null); setNewLootTable(null); }}
        onSubmit={async (data: CreateLootTableRequest) => {
          const response = await createLootTableMutation.mutateAsync(data);
          setSelectedLootTableId(null);
          setNewLootTable(null);
        }}
        isSubmitting={createLootTableMutation.isPending}
        lootTable={selectedTable || undefined}
      ></LootTableForm>
    </div>
  }

  return (
    <div className={`surface-base rounded-lg border border-theme-default ${className}`}>
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-xl font-semibold text-content-primary">Loot Table Management</h2>
            <p className="text-sm text-content-secondary mt-1">
              Create and control loot tables for your game
            </p>
          </div>
          <Button
            variant="primary"
            onClick={() => setNewLootTable(true)}
          >
            New Loot Table
          </Button>
        </div>

        {/* Loot Table List */}
        <div className="space-y-3">
          {lootTables.length === 0 ? (
            <div className="text-center py-8 text-content-secondary">
              <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <p>No loot tables created yet</p>
              <p className="text-sm">Create your first loot table</p>
            </div>
          ) : (
            lootTables.map((lootTable) => (
              <div  className="surface-base rounded-lg shadow-md p-6 md:flex justify-between items-start" key={lootTable.id}>
                <div className="flex gap-3 flex-grow">
                  <h3 className="text-xl font-bold text-content-primary mb-4">{lootTable.name}</h3>
                </div>
                <div className="flex gap-3">
                  <button
                    type="button"
                    aria-label="Edit"
                    onClick={() => setSelectedLootTableId(lootTable.id)}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
                  >
                    <PencilIcon className="h-5 w-5" />
                  </button>
                  <button
                    type="button"
                    aria-label="Delete"
                    onClick={() => deleteLootTable(lootTable.id)}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
                  >
                    <TrashIcon className="h-5 w-5" />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

    </div>
  );
}
