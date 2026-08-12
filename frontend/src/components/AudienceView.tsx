import { AllPrivateMessagesView } from './AllPrivateMessagesView';

interface AudienceViewProps {
  gameId: number;
}

/**
 * Audience view for private message traffic.
 *
 * Action submissions and results used to live here on a second sub-tab, which
 * duplicated the History tab's phase drill-down against a different set of
 * endpoints. They are now served from History for every role (see HistoryView),
 * leaving this view to the one thing History does not organise by phase:
 * private messages, which read by conversation.
 */
export function AudienceView({ gameId }: AudienceViewProps) {
  return <AllPrivateMessagesView gameId={gameId} />;
}
