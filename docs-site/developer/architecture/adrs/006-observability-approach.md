# ADR-006: Observability Approach

## Status
**Superseded (2026-06-04)** — the in-house-only decision was replaced by
OpenTelemetry + Grafana Cloud (backend) and Grafana Faro (frontend). The logging
design survives. See [Recent Architectural Evolution](#recent-architectural-evolution-june-2026).

## Context
ActionPhase requires comprehensive observability to support:
- Production debugging and troubleshooting
- Performance monitoring and optimization
- Business metrics and analytics
- Security monitoring and audit trails
- Capacity planning and resource optimization
- Development and testing visibility
- Compliance and operational requirements

The solution must provide actionable insights while maintaining minimal performance overhead and cost efficiency.

## Decision
We implemented a **Structured Observability Strategy** with three pillars:

**Logging**: Context-aware structured logging
- JSON-formatted logs with consistent field schemas
- Correlation ID propagation throughout request lifecycle
- Environment-appropriate log levels and handlers
- Centralized logging ready for aggregation systems

**Metrics**: Business and system performance metrics
- HTTP request metrics (latency, throughput, error rates)
- Business metrics (user actions, game state changes)
- Custom counters, gauges, and histogram tracking
- Real-time metrics endpoint for monitoring systems

**Tracing**: Request correlation and debugging
- Correlation IDs for end-to-end request tracking
- Request context propagation through middleware
- Operation timing and performance profiling
- Error correlation across system boundaries

## Alternatives Considered

### 1. APM Vendor Solution (New Relic, DataDog)
**Approach**: Third-party Application Performance Monitoring service.

**Pros**:
- Comprehensive out-of-the-box observability
- Advanced analytics and alerting capabilities
- Minimal implementation effort
- Professional support and SLA guarantees

**Cons**:
- Significant ongoing cost as system scales
- Vendor lock-in and dependency
- Limited customization for business-specific metrics
- Potential data privacy concerns

### 2. ELK Stack (Elasticsearch, Logstash, Kibana)
**Approach**: Self-hosted log aggregation and analysis platform.

**Pros**:
- Complete control over data and infrastructure
- Powerful search and analytics capabilities
- Cost-effective for large volumes
- Extensive customization options

**Cons**:
- Significant operational overhead to maintain
- Complex setup and configuration
- Resource-intensive infrastructure requirements
- Steep learning curve for effective usage

### 3. Prometheus + Grafana
**Approach**: Metrics-focused monitoring stack with time-series database.

**Pros**:
- Industry-standard metrics collection
- Excellent visualization and alerting
- Open-source with strong ecosystem
- Pull-based model suits container deployments

**Cons**:
- Metrics-only solution, needs separate logging
- Resource overhead for high-cardinality metrics
- Complex configuration for advanced features
- Limited out-of-the-box business intelligence

### 4. OpenTelemetry with Backend Agnostic
**Approach**: Vendor-neutral observability framework with pluggable backends.

**Pros**:
- Future-proof with vendor flexibility
- Comprehensive tracing, metrics, and logging
- Industry standard with broad ecosystem support
- Excellent correlation between signal types

**Cons**:
- Complex initial setup and configuration
- Resource overhead of full OpenTelemetry stack
- Overkill for current application size
- Requires expertise in multiple observability concepts

## Consequences

### Positive Consequences

**Development Productivity**:
- Structured logs make debugging faster and more effective
- Correlation IDs enable end-to-end request tracking
- Performance metrics identify bottlenecks early
- Development environment visibility improves testing

**Production Reliability**:
- Comprehensive error tracking with context
- Performance monitoring prevents degradation
- Business metrics enable data-driven decisions
- Security audit trail for compliance requirements

**Cost Efficiency**:
- In-memory metrics avoid external service costs
- Structured logging ready for any aggregation system
- Lightweight implementation with minimal performance impact
- Scalable approach that grows with application needs

**Operational Intelligence**:
- Real-time system health monitoring
- Business KPI tracking and trends
- User behavior analytics capabilities
- Performance optimization opportunities

### Negative Consequences

**Implementation Complexity**:
- Custom observability system requires ongoing maintenance
- Integration across all system components needed
- Proper correlation ID propagation requires discipline
- Testing observability features adds complexity

**Performance Overhead**:
- JSON log formatting has CPU cost
- In-memory metrics consume application memory
- Request tracing adds latency overhead
- Increased network traffic for log shipping

**Operational Requirements**:
- Need external systems for log aggregation
- Metrics storage and retention management
- Monitoring and alerting system setup
- Backup and disaster recovery for observability data

### Risk Mitigation Strategies

**Performance Management**:
- Configurable log levels to reduce overhead in production
- Bounded metric storage to prevent memory leaks
- Async logging to minimize request latency impact
- Performance benchmarking of observability components

**Data Management**:
- Structured log schemas for consistent parsing
- Metric retention policies to manage storage growth
- Correlation ID standards for reliable request tracking
- Data sanitization to prevent sensitive information leakage

**System Integration**:
- Standard interfaces for pluggable backends
- Environment-specific configuration management
- Graceful degradation when observability systems fail
- Health checks for observability infrastructure

## Implementation Details

### Logging Architecture
```go
// Context-aware logger with correlation IDs
type Logger struct {
    logger *slog.Logger
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
    if correlationID := GetCorrelationID(ctx); correlationID != "" {
        return &Logger{
            logger: l.logger.With("correlation_id", correlationID),
        }
    }
    if requestID := GetRequestID(ctx); requestID != "" {
        return &Logger{
            logger: l.logger.With("request_id", requestID),
        }
    }
    return l
}

// Operation logging with automatic timing
func (l *Logger) LogOperation(ctx context.Context, operation string, args ...any) func() {
    start := time.Now()
    l.WithContext(ctx).logger.Debug("Operation started",
        append([]any{"operation", operation}, args...)...)

    return func() {
        duration := time.Since(start)
        l.WithContext(ctx).logger.Info("Operation completed",
            "operation", operation,
            "duration_ms", duration.Milliseconds(),
            "status", "completed")
    }
}
```

### Metrics Collection
```go
// Business and system metrics tracking
type Metrics struct {
    // HTTP request metrics
    httpRequests    map[string]int64
    httpDurations   map[string][]float64
    httpErrors      map[string]int64

    // Business metrics
    counters    map[string]int64
    gauges      map[string]float64
    histograms  map[string][]float64

    totalRequests int64
    totalErrors   int64
    mu           sync.RWMutex
}

func (m *Metrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    routeKey := fmt.Sprintf("%s_%s", method, normalizeRoute(path))
    requestKey := fmt.Sprintf("%s_%d", routeKey, statusCode)

    m.httpRequests[requestKey]++
    m.totalRequests++

    durationMs := float64(duration.Nanoseconds()) / 1e6
    m.httpDurations[routeKey] = append(m.httpDurations[routeKey], durationMs)

    if statusCode >= 400 {
        m.httpErrors[routeKey]++
        m.totalErrors++
    }
}

// Business metrics interface
func (m *Metrics) IncrementCounter(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.counters[name]++
}

func (m *Metrics) SetGauge(name string, value float64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.gauges[name] = value
}
```

### Request Tracing Middleware
```go
func RequestTracingMiddleware(logger *Logger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Generate or extract correlation IDs
            correlationID := r.Header.Get("X-Correlation-ID")
            if correlationID == "" {
                correlationID = generateID("corr")
            }

            requestID := generateID("req")

            // Add to response headers
            w.Header().Set("X-Correlation-ID", correlationID)
            w.Header().Set("X-Request-ID", requestID)

            // Create enriched context
            ctx := WithCorrelationID(
                WithRequestID(r.Context(), requestID),
                correlationID,
            )

            // Log request start
            logger.WithContext(ctx).Info("HTTP request started",
                "method", r.Method,
                "path", r.URL.Path,
                "user_agent", r.UserAgent(),
                "remote_addr", r.RemoteAddr)

            // Process request with enriched context
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Metrics Endpoint
```go
func (h *ObservabilityHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    defer h.App.ObsLogger.LogOperation(ctx, "get_metrics")()

    snapshot := h.App.Observability.Metrics.GetSnapshot()

    response := map[string]interface{}{
        "http": map[string]interface{}{
            "total_requests": snapshot.TotalRequests,
            "total_errors":   snapshot.TotalErrors,
            "error_rate":     calculateErrorRate(snapshot),
            "requests_by_status": snapshot.HTTPRequests,
            "latency_percentiles": calculatePercentiles(snapshot.HTTPDurations),
        },
        "business": map[string]interface{}{
            "counters":   snapshot.Counters,
            "gauges":     snapshot.Gauges,
            "histograms": calculateHistogramStats(snapshot.Histograms),
        },
        "system": map[string]interface{}{
            "uptime_seconds": time.Since(h.App.Observability.StartTime).Seconds(),
            "memory_usage":   getMemoryUsage(),
        },
        "timestamp": time.Now().UTC(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### Structured Error Logging
```go
func (l *Logger) LogError(ctx context.Context, err error, operation string, args ...any) {
    errorArgs := append([]any{
        "error", err.Error(),
        "operation", operation,
        "stack_trace", getStackTrace(),
    }, args...)

    l.WithContext(ctx).logger.Error("Operation failed", errorArgs...)
}

// HTTP error logging with request context
func LogHTTPError(ctx context.Context, logger *Logger, err error, statusCode int) {
    correlationID := GetCorrelationID(ctx)
    requestID := GetRequestID(ctx)

    logger.Error("HTTP error response",
        "error", err.Error(),
        "status_code", statusCode,
        "correlation_id", correlationID,
        "request_id", requestID,
        "timestamp", time.Now().UTC())
}
```

## Log Schema Standards

### Standard Fields
```json
{
  "timestamp": "2025-08-07T10:30:00.123Z",
  "level": "INFO",
  "message": "Operation completed",
  "correlation_id": "corr_abc123",
  "request_id": "req_def456",
  "operation": "create_game",
  "duration_ms": 45,
  "status": "completed",
  "user_id": 123,
  "service": "actionphase-backend",
  "version": "1.2.3"
}
```

### Error Log Format
```json
{
  "timestamp": "2025-08-07T10:30:00.123Z",
  "level": "ERROR",
  "message": "Database connection failed",
  "correlation_id": "corr_abc123",
  "error": "connection timeout after 30s",
  "operation": "create_game",
  "stack_trace": "...",
  "context": {
    "user_id": 123,
    "game_id": 456,
    "attempt": 3
  }
}
```

## Business Metrics Definitions

### Game Engagement
- `games_created_total` - Total games created
- `games_active_current` - Currently active games
- `players_active_daily` - Daily active players
- `phases_completed_total` - Total completed game phases

### System Performance
- `http_requests_total` - HTTP requests by method/status
- `http_request_duration_ms` - Request latency percentiles
- `database_connections_active` - Active DB connections
- `memory_usage_bytes` - Application memory usage

### Error Tracking
- `errors_total` - Errors by type and severity
- `http_errors_total` - HTTP errors by status code
- `database_errors_total` - Database operation errors
- `authentication_failures_total` - Failed auth attempts

## Integration Patterns

### External Log Aggregation
```go
// Ready for Fluentd, Logstash, or similar
type LogShipper struct {
    endpoint string
    buffer   []LogEntry
    interval time.Duration
}

func (ls *LogShipper) Ship(ctx context.Context) error {
    // Batch ship logs to external system
    return ls.sendBatch(ctx, ls.buffer)
}
```

### Monitoring System Integration
```go
// Prometheus-compatible metrics endpoint
func (m *Metrics) PrometheusFormat() string {
    var metrics []string

    for name, value := range m.counters {
        metrics = append(metrics,
            fmt.Sprintf("actionphase_%s_total %d", name, value))
    }

    return strings.Join(metrics, "\n")
}
```

## Performance Characteristics

### Logging Overhead
- JSON marshaling: ~50ns per log entry
- Correlation ID lookup: ~10ns per request
- File I/O: Async to prevent blocking
- Memory usage: Bounded circular buffers

### Metrics Storage
- In-memory storage: ~1KB per metric type
- Bounded arrays prevent memory growth
- Read-heavy optimized data structures
- Periodic cleanup of stale metrics

### Tracing Impact
- Context propagation: ~5ns overhead
- ID generation: ~100ns per request
- Header processing: ~20ns per header
- Total request overhead: <1ms

## Future Enhancements

### Advanced Observability
- ~~**Distributed Tracing**: OpenTelemetry integration~~ — **done** (2026-06-04)
- ~~**Custom Dashboards**: Grafana integration for visualization~~ — **done** (Grafana Cloud)
- **Alerting**: Threshold-based alerting on key metrics
- **Log Analysis**: Machine learning for anomaly detection

### Business Intelligence
- **User Journey Tracking**: Complete user interaction flows
- **A/B Testing Integration**: Experiment metrics and analysis
- **Performance Profiling**: Code-level performance insights
- **Predictive Analytics**: Capacity planning and scaling predictions

## Recent Architectural Evolution (June 2026)

> **The core decision on this page was superseded on 2026-06-04** (commit
> `ab76cdd1`). The original ADR rejected both APM vendors ("significant ongoing
> cost", "vendor lock-in") and OpenTelemetry ("overkill for current application
> size") in favour of a custom in-house system. The project subsequently adopted
> **both**: a full OpenTelemetry stack exporting to **Grafana Cloud**.
>
> The text above is retained as the record of the original decision. What
> actually ships today is below.

### What replaced it

**Backend — OpenTelemetry → Grafana Cloud** (`backend/pkg/observability/`)

| File | Role |
|---|---|
| `tracing.go` | OTLP trace export |
| `metrics_otel.go` | `OTELMetrics`, OTLP metric push |
| `logging_otel.go` | slog → OTel log bridge (`otelslog`) |
| `middleware.go` | HTTP instrumentation (`otelhttp`) |

Postgres queries are traced via `otelpgx`. The OTLP endpoint is configured by
`OTELEndpoint` (`pkg/core/config.go`); auth uses the SDK's standard
`OTEL_EXPORTER_OTLP_HEADERS` env var.

**Frontend — Grafana Faro** (`frontend/src/lib/faro.ts`), which the original ADR
did not contemplate at all: `@grafana/faro-web-sdk`, `faro-react`, and
`faro-web-tracing` provide RUM, web-vitals, and browser tracing.

### What survived

The original logging design was **kept, not replaced**. `observability/logging.go`
still provides the `Logger` type, `WithContext`, and `LogOperation` exactly as
documented above, and correlation-ID propagation still works as described. The
in-memory `Metrics` struct also still exists and is still called from
`middleware.go:107` — it now runs *alongside* `OTELMetrics` rather than instead
of it.

### Corrections to the code samples above

- **`ObservabilityHandler` and the `/metrics` endpoint no longer exist.** The
  "Metrics Endpoint" sample documents a deleted API. There is **no `/metrics`
  route** in the backend; metrics reach Grafana by OTLP push only.
- `metrics_otel.go` constructs a `PrometheusHandler` whose comment claims it
  "serves /metrics in Prometheus text format for local scraping", but it is
  **never mounted on any route** — currently dead code.

### Cost rationale no longer applies

"In-memory metrics avoid external service costs" under *Cost Efficiency* is no
longer true: telemetry is shipped to a paid third-party platform. That trade-off
was made deliberately and should be re-documented in a new ADR if the reasoning
matters; this section only records that it changed.

## References
- [Structured Logging Best Practices](https://blog.golang.org/slog)
- [Observability Engineering](https://www.oreilly.com/library/view/observability-engineering/9781492076438/)
- [Prometheus Monitoring Guide](https://prometheus.io/docs/practices/)
- [OpenTelemetry Specification](https://opentelemetry.io/docs/)
