import React, { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useUserActionResults } from '../hooks/useActionResults';
import { useExpandedSet } from '../hooks/useExpandedSet';
import { Alert } from './ui';
import { CollapsibleMarkdown } from './CollapsibleMarkdown';
import { StagedPartPlaceholder } from './StagedPartPlaceholder';

interface ActionResultsListProps {
  gameId: number;
}

export const ActionResultsList: React.FC<ActionResultsListProps> = ({ gameId }) => {
  const { data: results, isLoading, error } = useUserActionResults(gameId);
  const expandedResults = useExpandedSet();
  const queryClient = useQueryClient();

  // A countdown reaching zero means "ask the server again", never "show it".
  const handlePartExpired = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
  }, [queryClient, gameId]);

  if (isLoading) {
    return (
      <div className="p-4">
        <p className="text-content-secondary">Loading your action results...</p>
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="danger">
        Error loading action results
      </Alert>
    );
  }

  if (!results || results.length === 0) {
    return (
      <div className="p-4 surface-raised border border-theme-default rounded">
        <p className="text-content-secondary">No action results yet.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h3 className="text-lg font-semibold text-content-primary">Your Action Results</h3>
      {results.map((result) => {
        const isExpanded = expandedResults.isExpanded(result.id);

        // `released_at` is the authority, not empty content: an ordinary result
        // has no released_at at all, and a locked part's content arrives blanked
        // from the server rather than absent.
        const isStagedPart = result.part_count !== undefined && result.part_count > 1;
        const isLocked = isStagedPart && !result.released_at;

        return (
          <div key={result.id} className="p-4 surface-base border border-theme-default rounded shadow-sm">
            <div className="flex justify-between items-start mb-2">
              <span className="text-sm text-content-tertiary">
                Phase {result.phase_number} - {result.phase_type}
              </span>
              {result.sent_at && (
                <span className="text-xs text-content-tertiary">
                  {new Date(result.sent_at).toLocaleString()}
                </span>
              )}
            </div>
            {isStagedPart && !isLocked && (
              <p className="text-xs font-medium text-content-secondary mb-2" data-testid={`staged-part-label-${result.id}`}>
                Part {result.part_number} of {result.part_count}
              </p>
            )}
            {isLocked ? (
              <StagedPartPlaceholder
                partNumber={result.part_number}
                partCount={result.part_count}
                unlocksAt={result.unlocks_at}
                onExpired={handlePartExpired}
              />
            ) : (
              <CollapsibleMarkdown
                content={result.content}
                fullWidth
                expanded={isExpanded}
                onExpandedChange={() => expandedResults.toggle(result.id)}
              />
            )}
            {result.gm_username && (
              <p className="text-xs text-content-tertiary mt-2">From: {result.gm_username}</p>
            )}
          </div>
        );
      })}
    </div>
  );
};
