# ActionPhase Documentation

Welcome to the ActionPhase documentation! This directory contains comprehensive technical documentation for the ActionPhase gaming platform.

## 📚 Documentation Index

### Getting Started
- **[Developer Onboarding Guide](getting-started/DEVELOPER_ONBOARDING.md)** - Get up and running in under 30 minutes
- **[Project README](../README.md)** - Project overview and quick start
- **[Environment Setup](../backend/README.md)** - Detailed setup instructions

### Architecture Documentation
- **[System Architecture](architecture/SYSTEM_ARCHITECTURE.md)** - Complete system design overview
- **[Component Interactions](architecture/COMPONENT_INTERACTIONS.md)** - How system components communicate
- **[Sequence Diagrams](architecture/SEQUENCE_DIAGRAMS.md)** - Visual flows for complex processes

### Architecture Decision Records (ADRs)
- **[ADR Index](adrs/README.md)** - Complete list of architectural decisions
- **[ADR-001: Technology Stack](adrs/001-technology-stack-selection.md)** - Go, React, PostgreSQL selection
- **[ADR-002: Database Design](adrs/002-database-design-approach.md)** - Hybrid relational-document approach
- **[ADR-003: Authentication Strategy](adrs/003-authentication-strategy.md)** - JWT + refresh token implementation
- **[ADR-004: API Design](adrs/004-api-design-principles.md)** - RESTful API design principles
- **[ADR-005: Frontend State Management](adrs/005-frontend-state-management.md)** - React Query + Context strategy
- **[ADR-006: Observability Approach](adrs/006-observability-approach.md)** - Structured logging and metrics
- **[ADR-007: Testing Strategy](adrs/007-testing-strategy.md)** - Multi-layer testing approach

### Development Guides
- **[Logging Standards](../.claude/reference/LOGGING_STANDARDS.md)** - Comprehensive logging guidelines

## 🚀 Quick Navigation

### New to ActionPhase?
Start with the **[Developer Onboarding Guide](getting-started/DEVELOPER_ONBOARDING.md)** to get your development environment running in minutes.

### Understanding the System?
Read the **[System Architecture](architecture/SYSTEM_ARCHITECTURE.md)** for a complete overview, then explore **[Component Interactions](architecture/COMPONENT_INTERACTIONS.md)** for detailed communication patterns.

### Making Architectural Changes?
Check existing **[Architecture Decision Records](adrs/README.md)** to understand past decisions, and create new ADRs for significant changes.

### Working on Features?
Reference the specific technology ADRs:
- **Backend Development**: [Technology Stack](adrs/001-technology-stack-selection.md), [Database Design](adrs/002-database-design-approach.md), [API Design](adrs/004-api-design-principles.md)
- **Frontend Development**: [Frontend State Management](adrs/005-frontend-state-management.md), [Technology Stack](adrs/001-technology-stack-selection.md)
- **Operations**: [Observability Approach](adrs/006-observability-approach.md), [Testing Strategy](adrs/007-testing-strategy.md)

## 📋 Documentation Status

| Component | Status | Last Updated |
|-----------|--------|--------------|
| Developer Onboarding | ✅ Complete | 2025-08-07 |
| MVP Status Tracking | ✅ Complete | 2025-10-15 |
| System Architecture | ✅ Complete | 2025-08-07 |
| Component Interactions | ✅ Complete | 2025-08-07 |
| Sequence Diagrams | ✅ Complete | 2025-08-07 |
| ADRs (001-007) | ✅ Complete | 2025-08-07 |
| State Management Guide | ✅ Complete | 2025-10-15 |
| API Documentation | 🔄 Planned | - |
| Database Schema Docs | 🔄 Planned | - |
| Deployment Guide | 🔄 Planned | - |

## 🎯 Documentation Goals

This documentation aims to:
- **Reduce onboarding time** for new developers from days to hours
- **Preserve architectural decisions** and their rationale for future reference
- **Improve code quality** through clear standards and patterns
- **Enable confident refactoring** with comprehensive system understanding
- **Support scaling** by documenting patterns and best practices

## 🔍 Finding Information

### By Role
- **New Developer**: [Onboarding Guide](DEVELOPER_ONBOARDING.md) → [System Architecture](architecture/SYSTEM_ARCHITECTURE.md)
- **Frontend Developer**: [ADR-005: State Management](adrs/005-frontend-state-management.md) → [Component Interactions](architecture/COMPONENT_INTERACTIONS.md)
- **Backend Developer**: [ADR-002: Database Design](adrs/002-database-design-approach.md) → [ADR-004: API Design](adrs/004-api-design-principles.md)
- **DevOps/SRE**: [ADR-006: Observability](adrs/006-observability-approach.md) → [Testing Strategy](adrs/007-testing-strategy.md)
- **Architect**: [ADR Index](adrs/README.md) → [System Architecture](architecture/SYSTEM_ARCHITECTURE.md)

### By Task
- **Checking project status**: [MVP Status & Development Plan](../.claude/planning/completed/MVP_STATUS.md)
- **Setting up development environment**: [Developer Onboarding](DEVELOPER_ONBOARDING.md)
- **Understanding data flow**: [Sequence Diagrams](architecture/SEQUENCE_DIAGRAMS.md)
- **Adding new features**: [Component Interactions](architecture/COMPONENT_INTERACTIONS.md)
- **Debugging issues**: [Observability Approach](adrs/006-observability-approach.md)
- **Writing tests**: [Testing Strategy](adrs/007-testing-strategy.md)
- **Database changes**: [Database Design ADR](adrs/002-database-design-approach.md)
- **Integrating state management**: [State Management Guide](../frontend/docs/STATE_MANAGEMENT.md)

## 📝 Contributing to Documentation

### Adding New Documentation
1. Follow the existing structure and naming conventions
2. Include a clear table of contents for longer documents
3. Use mermaid diagrams for visual representations
4. Cross-reference related documentation
5. Update this index when adding new documents

### Architecture Decision Records
When making significant architectural changes:
1. Create a new ADR using the template in `adrs/README.md`
2. Follow the ADR numbering convention (ADR-008, ADR-009, etc.)
3. Update the ADR index
4. Reference related ADRs and documentation

### Documentation Standards
- **Clear headings** with consistent hierarchy
- **Code examples** with proper syntax highlighting
- **Visual diagrams** for complex concepts
- **Cross-references** to related documentation
- **Practical examples** over theoretical explanations

## 🏗️ Architecture Overview

ActionPhase follows Clean Architecture principles with these key characteristics:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Clean Architecture                          │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (React/TS) ◄──HTTP/JSON──► Backend (Go/Chi)         │
│                                            ↕ SQL                │
│  • React Query Caching                PostgreSQL Database       │
│  • JWT Auto-refresh                                             │
│  • Component-based UI                                           │
├─────────────────────────────────────────────────────────────────┤
│                    Key Design Principles                        │
│  • Interface-first development (dependency inversion)          │
│  • Domain-driven design (clear business boundaries)            │
│  • API-first development (contract-driven)                     │
│  • Observability-first (structured logging & metrics)          │
│  • Testing pyramid (unit → integration → e2e)                  │
└─────────────────────────────────────────────────────────────────┘
```

## 🔮 Future Documentation Plans

- **API Reference**: OpenAPI-generated documentation
- **Database Schema Reference**: Auto-generated from migrations
- **Deployment Guides**: Production deployment and scaling
- **Performance Tuning**: Optimization guides and benchmarks
- **Security Guide**: Security best practices and threat model
- **Troubleshooting Guide**: Common issues and solutions

---

This documentation is a living resource that grows with the project. Keep it updated, accurate, and useful for the entire development team! 🚀
