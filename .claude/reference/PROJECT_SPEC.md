# ActionPhase: Play-by-Post RPG Platform - Project Specification

## Overview

ActionPhase is a specialized platform for running play-by-post RPG games that alternate between two distinct phases:
- **Common Room Phase**: Asynchronous, Reddit-style threaded discussions where players interact in-character
- **Action Phase**: Players submit private moves to the Game Master, who processes and publishes results

## Core Concept

Games follow a cyclical pattern:
1. **Common Room** (24-48 hours): Social interaction, planning, character development
2. **Action Phase** (deadline-based): Strategic move submission and GM resolution
3. **Repeat** until game completion

## User Roles & Permissions

### Game Master (GM)
- Full administrative control within their games
- Create and manage games through all phases
- Process player actions and publish results
- Access to all private communications within their games
- Create custom character sheet fields
- Set phase durations and deadlines
- Invite co-GMs with limited permissions

### Players
- Join games during recruitment phase only
- Create and manage characters (with GM approval)
- Participate in common room discussions (in-character)
- Submit private actions during action phases
- Access private messaging with other players
- Cannot leave games without GM involvement

### Audience Members
- Spectator role for community members
- Can view private conversations and limited GM notes
- Can play NPCs (Non-Player Characters) when assigned by GM
- Cannot participate as main characters in gameplay
- Useful for learning and community building

### Platform Administrators
- Site-wide rule enforcement
- Emergency game transfer/cancellation powers
- User management and moderation

## Game Lifecycle & States

### 1. Setup
- GM creates game framework
- Defines basic premise and rules
- Configures character sheet modules
- Private to GM only

### 2. Recruitment
- Game listed publicly for player discovery
- Players can apply to join
- GM reviews and accepts applications
- Game metadata visible (genre, dates, GM name)

### 3. Character Creation
- Closed to new players
- GM and accepted players collaborate on character creation
- Character approval process
- All characters must be approved before progression

### 4. In Progress
- Active gameplay alternating between Common Room and Action phases
- Full feature access for participants
- Audience members can observe

### 5. Paused/Completed/Cancelled
- End states with different implications
- Completed games become public archives
- All private data becomes publicly readable (actions, results, private messages)

## Character Management System

### Modular Character Sheets
Character sheets use a modular system where GMs can enable/disable components:

#### Current Modules
- **Public Profile**: Rich text character backstory and personality
- **Notes**: Player-private rich text notes about game and character
- **Skills**: Trained skills and special powers (name, free-text rank, category, description)
- **Inventory**: Items and equipment management
- **Numbers**: Named quantities and tracks (amount, optional max + display mode)

The three stat modules are GM-renameable per game; the names above are the
defaults. `Abilities` was a separate module until the character sheet refactor
retired it — it duplicated Skills, which is strictly more featured, so every
stat feature had to be built twice.

#### Phase 2 Modules
- **Ally Status**: Player-set feelings toward other characters
- **Skills/Stats**: Numeric attributes (Strength, Intelligence, etc.)
- **Relationships**: Connections to NPCs or world elements
- **Conditions/Status Effects**: Temporary modifiers
- **Location Tracking**: Current position in game world
- **Experience/Progression**: Character growth tracking
- **Resources**: Abstract resources (money, influence, reputation)
- **Secrets**: GM-only visible character information
- **Goals/Motivations**: Character objectives

#### Advanced Features
- **Module Interactions**: Modules can affect each other (e.g., talents affecting inventory capacity)
- **Custom Fields**: GMs can create additional custom fields
- **Character Templates**: Reusable character archetypes

### NPC (Non-Player Character) System
- **GM-Controlled NPCs**: Characters created and managed by Game Masters
- **Audience-Controlled NPCs**: NPCs assigned by GMs to audience members
- **NPC Permissions**: Limited character sheet access, can participate in all communications
- **Communication Access**: NPCs can join private messages and group chats with players
- **GM Oversight**: GMs receive notifications for all NPC communications (posts, private messages)
- **Assignment System**: GMs can assign/reassign NPC control to audience members
- **Character Distinction**: Clear visual distinction between Player Characters and NPCs

## Communication Systems

### Public Communication
- **Common Room Threads**: Reddit-style threaded discussions
- **In-Character Requirement**: All public posts must be in-character
- **Thread Structure**: One main thread per common room phase with nested replies

### Private Communication
- **Direct Messages**: 1-on-1 private conversations between players, NPCs, and GMs
- **Group Chats**: Player/NPC/GM created group conversations (invitation-based)
- **NPC Participation**: NPCs can initiate and participate in private communications
- **Game-Scoped**: All private communications tied to specific games
- **GM/Audience Visibility**: GMs and audience members can view all private messages
- **GM Notifications**: GMs receive alerts for any NPC communication activity

### Action Submissions
- **Rich Text**: Full formatting support for action descriptions
- **Asset Linking**: Direct links/references to character abilities and items
- **Private Until Game Completion**: Player actions remain private until entire game is finished
- **Private Results**: GM sends individual results to each player, also private until game completion
- **Clarification System**: GMs can request additional information from players

## Phase Management & Timing

### Common Room Phase
- **Duration**: 24, 36, or 48-hour windows (GM configurable)
- **Timing**: Typically scheduled over weekends
- **Extension Policy**: Generally discouraged, should be rare
- **Activities**: Character interaction, planning, social dynamics

### Action Phase
- **Submission Window**: GM-defined deadline for action submissions
- **Processing**: GM works privately to resolve all actions
- **Publication**: All results published simultaneously
- **Late Submissions**: GM-imposed penalties or exclusion from round

### Timing Features
- **Countdown Timers**: Visible to all participants
- **Custom Countdowns**: GMs can create arbitrary deadline timers
- **Timezone Handling**: All times displayed in user's local timezone
- **Phase Notifications**: In-app and optional email notifications

## Technical Architecture

### Backend Stack
- **Go**: Primary backend API server
- **PostgreSQL**: Relational data (users, games, characters, permissions)
- **Document Storage**: Rich text content (MongoDB/Elasticsearch)
- **JWT Authentication**: Token-based auth with refresh capabilities

### Frontend Stack
- **React + TypeScript**: Modern SPA framework
- **Tailwind CSS**: Utility-first styling with dark/light mode support
- **React Query**: Server state management and caching
- **Rich Text Editor**: Google Docs-style formatting toolbar
- **Responsive**: Desktop-first, mobile support in Phase 2

### Infrastructure
- **Database Backups**: Automated backups for PostgreSQL and document storage
- **File Storage**: S3-compatible storage for attachments/images
- **API-First**: RESTful API supporting SPA and future integrations
- **Discord Integration**: Webhook notifications (Phase 2)

## Feature Prioritization

### MVP (Phase 1)
**Core Functionality**
- User authentication and basic profiles
- Complete game lifecycle (all 6 states)
- Basic character sheets (Bio, Notes, Abilities, Inventory)
- NPC system with GM and audience member control
- Common room threaded discussions
- Action phase submissions with rich text and asset linking
- Private action results system (GM → individual players)
- Private messaging (1-on-1 and group)
- GM result publishing and phase transitions
- Countdown timers and phase notifications
- Audience member permissions including NPC control
- Admin emergency controls

**UI/UX Essentials**
- User onboarding with skip option
- Global and game-specific dashboards
- Game discovery with filtering (genre, GM, dates)
- Dark/light mode support
- High contrast for readability
- Clear read/unread indicators

### Phase 2
- Advanced character sheet modules
- Game archives with public access
- GM timeline view with character locations
- Character templates and approval workflow
- Email notifications
- Discord integration
- Mobile-responsive design
- Enhanced rich text (images, mentions, attachments)

### Phase 3
- Advanced GM tools (dice rollers, random events)
- Statistics tracking
- User tutorial system
- Advanced search and filtering
- Performance optimizations

## Documentation Requirements

### Integrated Help
- **Tooltips**: Context-sensitive help within the application
- **Help Overlays**: Interactive guided tours
- **In-App Reference**: Quick access to feature documentation

### External Documentation Site
- **New Player Guide**: Introduction to play-by-post RPGs and ActionPhase
- **GM Guide**: Platform tools and best practices
- **Feature Documentation**: Comprehensive platform reference
- **Technical API Documentation**: For developers and integrations

### Community Guidelines
- **Site Rules**: Integrated into main platform (not separate docs)
- **Moderation Policies**: Clear enforcement guidelines
- **Best Practices**: Community-driven gameplay recommendations

## Success Metrics

### Community Engagement
- Games created and completed per month
- Player retention across games
- Average game duration and satisfaction

### Platform Performance
- Page load times and responsiveness
- Uptime and reliability
- Data backup integrity

### User Experience
- Onboarding completion rates
- Feature adoption metrics
- Support ticket volume and resolution

## Security & Privacy

### Data Protection
- **Game Privacy**: Private communications remain private during active games
- **Post-Game Transparency**: All private data becomes public in completed games
- **User Data**: Standard privacy protections for personal information
- **Moderation Tools**: Admin access for rule enforcement

### Security Measures
- **Authentication**: Secure JWT implementation with refresh tokens
- **Authorization**: Role-based permissions per game
- **Data Validation**: Input sanitization and validation
- **Backup Security**: Encrypted backups with retention policies

---

This specification serves as the comprehensive guide for developing ActionPhase from MVP through full feature completion.
