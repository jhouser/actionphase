import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ChevronDown, ChevronUp, ExternalLink } from 'lucide-react';
import { Card, CardBody, Button, Badge } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { useExpandedSet } from '../hooks/useExpandedSet';
import type { ActionResult } from '../types/phases';

interface RecentResultsSectionProps {
  gameId: number;
  results: ActionResult[];
  previousPhaseId: number;
  previousPhaseTitle: string;
}

/**
 * RecentResultsSection Component
 *
 * Displays recent action results at the top of the Common Room when:
 * - Current phase is common_room
 * - Previous phase was action with published results
 *
 * Features:
 * - Collapsible results with expand/collapse state
 * - Auto-collapse after first view (tracked in localStorage)
 * - "View Full Results" link to History tab
 * - Individual result cards with expand/collapse
 */
export function RecentResultsSection({
  gameId,
  results,
  previousPhaseId,
  previousPhaseTitle
}: RecentResultsSectionProps) {
  const navigate = useNavigate();

  // Check if user has already viewed these results
  const storageKey = `results-viewed-${gameId}-${previousPhaseId}`;
  const hasViewed = localStorage.getItem(storageKey) === 'true';

  // Start collapsed if user has already viewed
  const [isExpanded, setIsExpanded] = useState(!hasViewed);

  // Track individual result expansion state (all collapsed by default)
  const expandedResults = useExpandedSet();

  // Mark as viewed when expanded
  useEffect(() => {
    if (isExpanded && !hasViewed) {
      localStorage.setItem(storageKey, 'true');
    }
  }, [isExpanded, hasViewed, storageKey]);


  const handleViewFullResults = () => {
    // Navigate to History tab with phase selected
    navigate(`/games/${gameId}?tab=history&phase=${previousPhaseId}`);
  };

  if (results.length === 0) {
    return null;
  }

  return (
    <Card variant="elevated" padding="md" className="mb-6 border-2 border-interactive-primary">
      {/* Header */}
      <div
        className="flex items-center justify-between cursor-pointer"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-3">
          <div className="text-2xl">📋</div>
          <div>
            <h3 className="text-lg font-semibold text-content-primary">
              Recent Action Results
            </h3>
            <p className="text-sm text-content-secondary">
              From {previousPhaseTitle}
            </p>
          </div>
          <Badge variant="primary">
            {results.length} {results.length === 1 ? 'result' : 'results'}
          </Badge>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              handleViewFullResults();
            }}
            className="flex items-center gap-2"
          >
            <span>View Full Results</span>
            <ExternalLink className="w-4 h-4" />
          </Button>

          <Button variant="ghost" size="sm" className="p-2">
            {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
          </Button>
        </div>
      </div>

      {/* Results Content */}
      {isExpanded && (
        <CardBody className="mt-4 space-y-3">
          {results.map((result) => {
            const isResultExpanded = expandedResults.isExpanded(result.id);

            return (
              <Card
                key={result.id}
                variant="default"
                padding="sm"
                className="border border-border-primary"
              >
                <div
                  className="flex items-start justify-between cursor-pointer"
                  onClick={() => expandedResults.toggle(result.id)}
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-medium text-content-primary">
                        {result.username || 'Result'}
                      </span>
                      {result.sent_at && (
                        <span className="text-xs text-content-tertiary">
                          {new Date(result.sent_at).toLocaleString()}
                        </span>
                      )}
                    </div>

                    {/* Preview (first 150 characters) */}
                    {!isResultExpanded && (
                      <p className="text-sm text-content-secondary line-clamp-2">
                        {result.content.slice(0, 150)}
                        {result.content.length > 150 ? '...' : ''}
                      </p>
                    )}
                  </div>

                  <Button variant="ghost" size="sm" className="p-2">
                    {isResultExpanded ? (
                      <ChevronUp className="w-4 h-4" />
                    ) : (
                      <ChevronDown className="w-4 h-4" />
                    )}
                  </Button>
                </div>

                {/* Full Content */}
                {isResultExpanded && (
                  <div className="mt-3 pt-3 border-t border-border-primary">
                    <MarkdownPreview content={result.content} />
                  </div>
                )}
              </Card>
            );
          })}

          <div className="text-center pt-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleViewFullResults}
              className="flex items-center gap-2 mx-auto"
            >
              <span>View Full Results in History</span>
              <ExternalLink className="w-4 h-4" />
            </Button>
          </div>
        </CardBody>
      )}
    </Card>
  );
}
