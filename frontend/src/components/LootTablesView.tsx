import { Button, Modal } from './ui';
import { useLootTableManagement } from '@/hooks/useLootTablemanagement';
import { LootTableForm, type EditLootTable } from './loot-tables/LootTableForm';
import { useUrlParam } from '@/hooks/useUrlParam';
import { PencilIcon, TrashIcon } from '@heroicons/react/24/outline';
import { useState } from 'react';
import { formatUTCForDisplay } from '@/utils/timezone';
import type { LootTable } from '@/types/games';

interface LootTablesProps {
  gameId: number;
  className?: string;
}


/**
 * Whether a table has been edited since it was created.
 *
 * The migration backfilled updated_at from created_at and the insert defaults
 * both to NOW(), so an untouched table has the two within microseconds of each
 * other — but not always byte-identical, which rules out a string compare. A
 * one-second tolerance is far below any real edit interval.
 */
function wasUpdated(lootTable: LootTable): boolean {
  if (!lootTable.updated_at || !lootTable.created_at) return false;
  const delta = new Date(lootTable.updated_at).getTime() - new Date(lootTable.created_at).getTime();
  return Number.isFinite(delta) && delta > 1000;
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
    updateLootTableMutation,
    updateLootTableContentsMutation,
    deleteLootTableMutation
  } = useLootTableManagement(gameId);

  const [checkDeleteLootTable, setCheckDeleteLootTable] = useState(false);

  const selectedTable = lootTables.find(t => t.id === selectedLootTableId);

  const [lootTableToDelete, setLootTableToDelete] = useState<number | undefined>();

  const deleteLootTable = async (lootTableId: number) => {
    setLootTableToDelete(undefined);
    if(!checkDeleteLootTable) {
      setLootTableToDelete(lootTableId);
      setCheckDeleteLootTable(true);
      return;
    }
    setCheckDeleteLootTable(false);
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

  if ((selectedLootTableId !== null && selectedTable) || newLootTable) {
    return <div>
        <Button
          variant="ghost"
          onClick={() => {
            if (selectedLootTableId !== null) {
              setSelectedLootTableId(null); 
            }
            if (newLootTable !== null) {
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
        onClose={() => {
            if (selectedLootTableId !== null) {
              setSelectedLootTableId(null); 
            }
            if (newLootTable !== null) {
              setNewLootTable(null); 
            } }}
        onSubmit={async (data: EditLootTable) => {
          if (!data.id){
            await createLootTableMutation.mutateAsync({
              name: data.name,
              items: data.items
            });
          }
          else {
            if (selectedTable?.name !== data.name){
              await updateLootTableMutation.mutateAsync({
                id: data.id,
                name: data.name
              });
            }
            if (data.itemsChanged) {
              await updateLootTableContentsMutation.mutateAsync({
                id: data.id,
                items: data.items || []
              });
              
            }
          }
          if (selectedLootTableId !== null) {
            setSelectedLootTableId(null); 
          }
          if (newLootTable !== null) {
            setNewLootTable(null); 
          }
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
            // surface-raised + border-theme-default is the site-wide pattern for an
            // item nested inside a section (see GameStatsView, HistoryView). These
            // cards previously used surface-base — the same value as the container
            // behind them — so only a shadow separated them, which is nearly
            // invisible in dark mode and left the list reading as one block.
            lootTables.map((lootTable) => (
              <div
                className="surface-raised border border-theme-default rounded-lg p-4 flex flex-wrap gap-3 justify-between items-center"
                key={lootTable.id}
              >
                <div className="flex flex-col gap-1 flex-grow min-w-0">
                  {/* min-w-0 + break-words so a long name wraps instead of pushing
                      the action buttons off the card. */}
                  <h3 className="text-lg font-semibold text-content-primary min-w-0 break-words">
                    {lootTable.name}
                  </h3>
                  <p className="text-xs text-content-tertiary">
                    Created {formatUTCForDisplay(lootTable.created_at, 'MMM d, yyyy')}
                    {/* updated_at is set to created_at on insert and only moves when
                        the table is renamed, so showing both when they match would
                        just be the same date twice. */}
                    {wasUpdated(lootTable) && (
                      <> · Updated {formatUTCForDisplay(lootTable.updated_at, 'MMM d, yyyy')}</>
                    )}
                  </p>
                </div>
                <div className="flex gap-3 shrink-0">
                  {/* Name the table in the label: every row otherwise announces a
                      bare "Edit"/"Delete", which is ambiguous in a list. */}
                  <button
                    type="button"
                    aria-label={`Edit ${lootTable.name}`}
                    onClick={() => setSelectedLootTableId(lootTable.id)}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
                  >
                    <PencilIcon className="h-5 w-5" />
                  </button>
                  <button
                    type="button"
                    aria-label={`Delete ${lootTable.name}`}
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
        
        {/* Delete Character Confirmation Modal */}
        {checkDeleteLootTable && (
          <Modal
            isOpen={true}
            onClose={() => { setLootTableToDelete(undefined); setCheckDeleteLootTable(false); }}
            title="Delete Loot Table?"
          >
            <div className="space-y-4">
              <p className="text-content-primary">
                Are you sure you want to delete this loot table?
              </p>
              <p className="text-sm text-content-secondary">
                This action cannot be undone. All content within the loot table will be lost.
              </p>
  
              <div className="flex justify-end space-x-2">
                <Button
                  variant="secondary"
                  onClick={() => { setLootTableToDelete(undefined); setCheckDeleteLootTable(false); }}
                  disabled={deleteLootTableMutation.isPending}
                >
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  onClick={() => {if (lootTableToDelete) deleteLootTable(lootTableToDelete); }}
                  loading={deleteLootTableMutation.isPending}
                  data-testid="confirm-delete-character-button"
                >
                  Delete Loot Table
                </Button>
              </div>
            </div>
          </Modal>
        )}
      </div>

    </div>
  );
}
