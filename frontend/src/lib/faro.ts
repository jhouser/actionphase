import {
  initializeFaro,
  getWebInstrumentations,
  type Faro,
} from '@grafana/faro-web-sdk';
import { TracingInstrumentation } from '@grafana/faro-web-tracing';
import { ReactIntegration, createReactRouterV6DataOptions } from '@grafana/faro-react';
import { matchRoutes } from 'react-router-dom';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { enrichHttpSpan } from './tracing/httpRouteEnrichment';
import { initLongAnimationFrames } from './tracing/longAnimationFrames';
import { shouldDropWebVital } from './tracing/webVitalsSanity';

let faroInstance: Faro | null = null;

export function initFaro(): void {
  const enabled = import.meta.env.VITE_FARO_ENABLED === 'true';
  const url = import.meta.env.VITE_FARO_URL;

  if (!enabled || !url) {
    return;
  }

  faroInstance = initializeFaro({
    url,
    app: {
      name: import.meta.env.VITE_FARO_APP_NAME ?? 'actionphase',
      version: '1.0.0',
      environment: import.meta.env.VITE_ENVIRONMENT ?? 'development',
    },
    // Drops web-vitals samples from stalled/backgrounded navigations (seen on
    // iOS Safari: the timing clock freezes mid-request, then resumes and
    // reports a false multi-second ttfb/fcp/lcp) before they poison Grafana's
    // per-page aggregates. See webVitalsSanity.ts for the policy rationale.
    beforeSend: (item) => (shouldDropWebVital(item) ? null : item),
    instrumentations: [
      // getWebInstrumentations includes UserActionInstrumentation, which is opt-in
      // per element: a user action is only tracked when the clicked/keyed element (or
      // an ancestor) carries `data-faro-user-action-name`. Annotate the interactive
      // element itself (the <button>/<Button>, not an inner <svg>) with a kebab-case
      // verb-noun name. Name distinct surfaces distinctly so attribution can tell them
      // apart. Currently annotated: nav (`open-notifications`, `open-user-menu`,
      // `open-mobile-menu`); action phase (`submit-action`, `open-character-sheet`);
      // common room (`create-post`, `submit-comment`); private conversations
      // (`send-private-message`, `open-conversation`, `start-conversation`); GM loop
      // (`activate-phase`, `create-phase`, `create-character`, `create-action-result`).
      // This gives INP's `interaction_target` and Faro user-action events a stable human
      // name instead of an ambiguous CSS selector (`svg.h-6.w-6>path`), and tags HTTP
      // spans fired during the action with that name. Add new annotations opportunistically
      // as slow INP captures name new targets; do not blanket-annotate.
      ...getWebInstrumentations({
        captureConsole: true,
        enablePerformanceInstrumentation: false,
      }),
      new TracingInstrumentation({
        // Enrich HTTP client spans with a low-cardinality `http.route`, a
        // `{METHOD} {route}` name, and `app.game.id` so Grafana Application
        // Observability groups by endpoint instead of by bare HTTP verb.
        // axios uses XHR; the fetch hook covers any direct `fetch()` calls.
        instrumentationOptions: {
          xhrInstrumentationOptions: {
            applyCustomAttributesOnSpan: (span) => enrichHttpSpan(span),
          },
          fetchInstrumentationOptions: {
            applyCustomAttributesOnSpan: (span) => enrichHttpSpan(span),
          },
        },
      }),
      new ReactIntegration({
        router: createReactRouterV6DataOptions({
          matchRoutes,
        }),
      }),
    ],
  });

  // Register document-load separately rather than via TracingInstrumentation's
  // `instrumentations` array: overriding that array would drop Faro's own
  // FaroXhrInstrumentation glue and its self-ignore guard (Faro auto-ignores its
  // collector URL to avoid tracing its own exports). TracingInstrumentation's
  // provider.register() has already set the global tracer provider, so
  // DocumentLoadInstrumentation binds to it and exports through Faro's pipeline.
  // It emits a `documentLoad` waterfall span (DNS/TCP/TLS/TTFB/DOM) once per full
  // page load — not per SPA route change.
  registerInstrumentations({
    instrumentations: [new DocumentLoadInstrumentation()],
  });

  // Long Animation Frames: a PerformanceObserver-based detector for main-thread
  // blocks, attributed to the script source + function. Reads from the Faro instance
  // above rather than the `instrumentations` array (it's an observer, not an OTel
  // instrumentation). Chromium-only; no-ops elsewhere.
  initLongAnimationFrames(faroInstance);
}

// pushError sends an error to Faro if it is initialized.
// Call this from ErrorBoundary.componentDidCatch.
export function pushError(error: Error, context?: Record<string, string>): void {
  faroInstance?.api.pushError(error, { context });
}

// setFaroUser attaches the current user's id to Faro's session metadata, which
// its FaroMetaAttributesSpanProcessor copies onto every span as `user.id` — giving
// traces business context to filter by. We deliberately send only the id, not
// email/username, to keep PII out of Grafana. No-op if Faro isn't initialized.
export function setFaroUser(userId: number): void {
  faroInstance?.api.setUser({ id: String(userId) });
}

// clearFaroUser removes user metadata on logout so subsequent anonymous spans
// aren't attributed to the previous user.
export function clearFaroUser(): void {
  faroInstance?.api.resetUser();
}

