# State Management - Comprehensive Guide

**Last Updated:** October 2025 · **Verified:** 2026-08-26

> Uses **TanStack Query v5** (`^5.84.1`). Query-filter methods take an object —
> `invalidateQueries({ queryKey: [...] })` — not the v4 positional array form.

This is the single source of truth for state management in the ActionPhase frontend application.

## Table of Contents

1. [Overview](#overview)
2. [State Management Strategy](#state-management-strategy)
3. [Architecture](#architecture)
4. [Core Components](#core-components)
   - [AuthContext](#authcontext)
   - [GameContext](#gamecontext)
   - [Custom Hooks](#custom-hooks)
5. [Usage Patterns](#usage-patterns)
6. [Migration Guide](#migration-guide)
7. [Best Practices](#best-practices)
8. [React Query Integration](#react-query-integration)
9. [Performance](#performance)
10. [Testing](#testing)
11. [Debugging](#debugging)
12. [TypeScript Support](#typescript-support)
13. [Quick Reference](#quick-reference)

---

## Overview

ActionPhase uses a **Hybrid State Management Strategy** with specialized tools optimized for different types of state:

- **Server State**: React Query (TanStack Query) for API data caching
- **Authentication State**: Centralized AuthContext with React Query integration
- **Game Context**: GameContext for game-specific state and permissions
- **UI State**: Local React useState/useReducer for component-specific state
- **Global UI State**: React Context (sparingly) for truly global UI concerns

### October 2025 Refactor

A comprehensive refactor in October 2025 consolidated duplicated user identity, game context, and permissions management:

**Problems Solved:**
- 15+ components independently fetching user data
- 60-70% duplicate API calls
- Client-side JWT decoding (security risk)
- Race conditions from inconsistent loading states
- Scattered permission calculations across components

**Results:**
- 60-70% reduction in API calls
- Zero client-side JWT decoding
- Single source of truth for authentication
- Consistent loading states
- Centralized permission logic

---

## State Management Strategy

### 1. Server State: React Query (TanStack Query)

**For all API communication and server data caching**

React Query provides:
- Automatic background refetching
- Intelligent caching with stale-while-revalidate
- Optimistic updates for mutations
- Automatic retry logic
- Request deduplication
- Built-in loading and error states

**Configuration:**
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes for user data
      retry: (failureCount, error) => {
        if (error.status === 401) return false; // Don't retry auth errors
        return failureCount < 3;
      },
    },
    mutations: {
      retry: false, // Don't retry mutations by default
    },
  },
});
```

### 2. Authentication State: AuthContext + React Query

**Centralized authentication with single source of truth**

Features:
- JWT token management
- User session state (id, username, email)
- Login/logout orchestration
- Automatic token refresh
- **isCheckingAuth flag** to prevent race conditions

**Key Benefit:** User data is fetched once and cached, eliminating duplicate `/auth/me` calls across components.

### 3. Game Context: GameContext

**Game-specific state and permissions**

Provides:
- Game details with React Query caching
- Participant management
- User's controllable characters
- Automatic role calculation (gm, player, co_gm, audience, none)
- Permission helpers (isGM, canEditGame, canManagePhases)
- Character ownership checking

### 4. UI State: React useState + useReducer

**Component-local state for forms and interactions**

Use for:
- Form inputs and validation
- Modal visibility
- Loading indicators
- Temporary UI state
- Component-specific toggles

### 5. Global UI State: React Context (Sparingly)

**Only for truly global state**

Appropriate for:
- Theme preferences
- User settings
- Notification management
- Navigation state

**Anti-pattern:** Don't use Context for server data that should be cached by React Query.

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        React Application                             │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │              QueryClientProvider (React Query)             │    │
│  │                                                            │    │
│  │  ┌──────────────────────────────────────────────────────┐ │    │
│  │  │              AuthProvider (Context)                  │ │    │
│  │  │                                                      │ │    │
│  │  │  • Fetches & stores current user                    │ │    │
│  │  │  • Manages authentication state                     │ │    │
│  │  │  • Provides user ID, username, email                │ │    │
│  │  │  • Available to ALL components                      │ │    │
│  │  │                                                      │ │    │
│  │  │  ┌────────────────────────────────────────────────┐ │ │    │
│  │  │  │         BrowserRouter (Routes)              │ │ │    │
│  │  │  │                                             │ │ │    │
│  │  │  │  ┌──────────────────────────────────────┐  │ │ │    │
│  │  │  │  │      Public Routes                   │  │ │ │    │
│  │  │  │  │  • Login, Register, Public Games     │  │ │ │    │
│  │  │  │  │  • Use useAuth() for state           │  │ │ │    │
│  │  │  │  └──────────────────────────────────────┘  │ │ │    │
│  │  │  │                                             │ │ │    │
│  │  │  │  ┌──────────────────────────────────────┐  │ │ │    │
│  │  │  │  │   Game-Specific Routes               │  │ │ │    │
│  │  │  │  │   (Wrapped with GameProvider)        │  │ │ │    │
│  │  │  │  │                                       │  │ │ │    │
│  │  │  │  │  ┌─────────────────────────────────┐ │  │ │ │    │
│  │  │  │  │  │   GameProvider (Context)        │ │  │ │ │    │
│  │  │  │  │  │                                 │ │  │ │ │    │
│  │  │  │  │  │  • Fetches game details        │ │  │ │ │    │
│  │  │  │  │  │  • Fetches participants        │ │  │ │ │    │
│  │  │  │  │  │  • Fetches user's characters   │ │  │ │ │    │
│  │  │  │  │  │  • Calculates permissions      │ │  │ │ │    │
│  │  │  │  │  │  • Provides role & helpers     │ │  │ │ │    │
│  │  │  │  │  │                                 │ │  │ │ │    │
│  │  │  │  │  │  ┌───────────────────────────┐ │ │  │ │ │    │
│  │  │  │  │  │  │  Game Page Components     │ │ │  │ │ │    │
│  │  │  │  │  │  │                           │ │ │  │ │ │    │
│  │  │  │  │  │  │  • Use useGameContext()   │ │ │  │ │ │    │
│  │  │  │  │  │  │  • Access game, role, etc │ │ │  │ │ │    │
│  │  │  │  │  │  │  • Check permissions      │ │ │  │ │ │    │
│  │  │  │  │  │  └───────────────────────────┘ │ │  │ │ │    │
│  │  │  │  │  └─────────────────────────────────┘ │  │ │ │    │
│  │  │  │  └──────────────────────────────────────┘  │ │ │    │
│  │  │  │                                             │ │ │    │
│  │  │  │  ┌──────────────────────────────────────┐  │ │ │    │
│  │  │  │  │   Other Routes                       │  │ │ │    │
│  │  │  │  │   (Use hooks without context)        │  │ │ │    │
│  │  │  │  │                                       │  │ │ │    │
│  │  │  │  │  • Use useGamePermissions(gameId)    │  │ │ │    │
│  │  │  │  │  • Use useUserCharacters(gameId)     │  │ │ │    │
│  │  │  │  │  • Use useCharacterOwnership(gameId) │  │ │ │    │
│  │  │  │  └──────────────────────────────────────┘  │ │ │    │
│  │  │  └────────────────────────────────────────────┘ │ │    │
│  │  └──────────────────────────────────────────────────┘ │    │
│  └────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow Diagram

#### Authentication Flow

```
User Login
    │
    ├──> AuthContext.login()
    │       │
    │       ├──> API: POST /api/v1/auth/login
    │       │       │
    │       │       └──> Returns JWT token
    │       │
    │       ├──> Store token in localStorage
    │       │
    │       ├──> Set isAuthenticated = true
    │       │
    │       └──> Trigger user data fetch
    │               │
    │               ├──> API: GET /api/v1/auth/me
    │               │       │
    │               │       └──> Returns { id, username, email }
    │               │
    │               └──> Set currentUser in context
    │
    └──> All components can now access currentUser via useAuth()
```

#### Game Context Flow

```
<GameProvider gameId={123}>
    │
    ├──> Fetch game details
    │       │
    │       └──> API: GET /api/v1/games/123/details
    │               │
    │               └──> Cached by React Query (30s)
    │
    ├──> Fetch participants
    │       │
    │       └──> API: GET /api/v1/games/123/participants
    │               │
    │               └──> Cached by React Query (30s)
    │
    ├──> Fetch user's characters
    │       │
    │       └──> API: GET /api/v1/games/123/characters/controllable
    │               │
    │               └──> Cached by React Query (30s)
    │
    ├──> Calculate user role
    │       │
    │       ├──> If user_id === gm_user_id → role = 'gm'
    │       ├──> Else find in participants
    │       └──> Default to 'none'
    │
    ├──> Calculate permissions
    │       │
    │       ├──> isGM = (role === 'gm')
    │       ├──> isParticipant = (role in ['player', 'co_gm'])
    │       └──> canEditGame = isGM
    │
    └──> Provide context to children
            │
            └──> Components use useGameContext()
                    │
                    ├──> Access game data
                    ├──> Check permissions
                    └──> Check character ownership
```

---

## Core Components

### AuthContext

**Location:** `frontend/src/contexts/AuthContext.tsx`

#### Features

- Automatically fetches current user data from `/api/v1/auth/me` on mount
- Stores complete user object (id, username, email)
- Provides authentication state management
- Handles login, registration, and logout
- Maintains backward compatibility with existing code
- Fully integrated with React Query for caching

#### Interface

```typescript
interface AuthContextValue {
  currentUser: User | null;           // User object with id, username, email
  isAuthenticated: boolean;           // Whether user is authenticated
  isLoading: boolean;                 // Auth action in progress
  isCheckingAuth: boolean;            // Initial auth check (CRITICAL)
  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
  error: Error | null;                // Any auth errors
}
```

#### Implementation

```typescript
export function AuthProvider({ children }: { children: ReactNode }) {
  // Single query for authentication state
  const { data: isAuthenticated, isLoading: isCheckingAuth } = useQuery({
    queryKey: ['auth'],
    queryFn: () => !!apiClient.getAuthToken(),
    staleTime: 0,
    refetchOnWindowFocus: true,
  });

  // Single query for user data
  const { data: currentUser, isLoading: isLoadingUser } = useQuery({
    queryKey: ['currentUser'],
    queryFn: async () => {
      const response = await apiClient.getCurrentUser();
      return response.data;
    },
    enabled: isAuthenticated === true,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  });

  const isLoading = isCheckingAuth || isLoadingUser;

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}
```

#### Usage

```typescript
import { useAuth } from '../contexts/AuthContext';

function MyComponent() {
  const {
    currentUser,      // User object with id, username, email
    isAuthenticated,  // boolean
    isLoading,        // Auth action in progress
    isCheckingAuth,   // Initial auth check
    login,            // Login function
    register,         // Register function
    logout,           // Logout function
    error             // Any auth errors
  } = useAuth();

  // User ID is immediately available after login
  console.log('Current user ID:', currentUser?.id);
  console.log('Current username:', currentUser?.username);

  return (
    <div>
      {isAuthenticated ? (
        <p>Welcome, {currentUser?.username}!</p>
      ) : (
        <p>Please log in</p>
      )}
    </div>
  );
}
```

#### Setup

Wrap your application with `AuthProvider`:

```typescript
// In App.tsx or main entry point
import { AuthProvider } from './contexts/AuthContext';

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        {/* Your app routes */}
      </AuthProvider>
    </QueryClientProvider>
  );
}
```

#### Critical: isCheckingAuth Flag

**ALWAYS use `isCheckingAuth` to prevent premature rendering**

```typescript
// ❌ BAD: Button may render before auth state is known
{!isGM && game.state === 'recruitment' && (
  <button onClick={handleApply}>Apply to Join</button>
)}

// ✅ GOOD: Wait for auth state before rendering
{!isGM && !isCheckingAuth && game.state === 'recruitment' && (
  <button onClick={handleApply}>Apply to Join</button>
)}
```

**Why:** Without `isCheckingAuth`, buttons/UI may briefly appear before auth is confirmed, causing UX issues and potential bugs (e.g., GM seeing "Apply to Join" on their own game).

#### Getting User ID

```typescript
// ✅ CORRECT: Use nullish coalescing
const currentUserId = currentUser?.id ?? null;

// ✅ CORRECT: Direct comparison
const isGM = game.gm_user_id === currentUser?.id;

// ❌ WRONG: Don't decode JWT client-side
const decoded = decodeJWT(token);  // NEVER DO THIS
```

---

### GameContext

**Location:** `frontend/src/contexts/GameContext.tsx`

#### Features

- Provides game details using React Query
- Fetches and manages game participants
- Fetches user's controllable characters
- Automatically calculates user's role (gm, player, co_gm, audience, none)
- Provides permission helpers (isGM, isParticipant, canEditGame)
- Includes character ownership checker function
- Offers `refetchGameData()` for manual refresh
- Uses React Query for efficient caching

#### Interface

```typescript
interface GameContextValue {
  gameId: number;
  game: GameWithDetails | null;
  participants: GameParticipant[];
  userRole: UserGameRole;              // 'gm' | 'player' | 'co_gm' | 'audience' | 'none'
  isGM: boolean;
  isCoGM: boolean;
  isPlayer: boolean;
  isAudience: boolean;
  isParticipant: boolean;              // player or co_gm
  canEditGame: boolean;
  canManagePhases: boolean;            // GM or co-GM
  canViewAllActions: boolean;          // GM or co-GM
  userCharacters: Character[];         // User's controllable characters
  isUserCharacter: (characterId: number) => boolean;
  isLoadingGame: boolean;
  isLoadingParticipants: boolean;
  isLoadingCharacters: boolean;
  refetchGameData: () => Promise<void>;
}
```

#### Usage

```typescript
import { GameProvider, useGameContext } from '../contexts/GameContext';

function GameDetailsPage({ gameId }: { gameId: number }) {
  return (
    <GameProvider gameId={gameId}>
      <GameContent />
    </GameProvider>
  );
}

function GameContent() {
  const {
    gameId,           // number
    game,             // GameWithDetails | null
    participants,     // GameParticipant[]
    userRole,         // 'gm' | 'player' | 'co_gm' | 'audience' | 'none'
    isGM,             // boolean
    isParticipant,    // boolean (player or co_gm)
    canEditGame,      // boolean
    userCharacters,   // Character[] - User's controllable characters
    isUserCharacter,  // (characterId: number) => boolean
    isLoadingGame,    // boolean
    isLoadingParticipants, // boolean
    isLoadingCharacters,   // boolean
    refetchGameData,  // () => Promise<void>
  } = useGameContext();

  return (
    <div>
      {isGM && <button onClick={editGame}>Edit Game</button>}
      {isParticipant && <p>You are a participant</p>}

      {userCharacters.map(char => (
        <div key={char.id}>{char.name}</div>
      ))}
    </div>
  );
}
```

#### Optional Usage

For components that might be used outside of a GameContext:

```typescript
import { useOptionalGameContext } from '../contexts/GameContext';

function MyComponent() {
  const gameContext = useOptionalGameContext();

  if (gameContext) {
    // Use game context
    const { isGM } = gameContext;
  } else {
    // Handle case where no game context
  }
}
```

---

### Custom Hooks

#### useGamePermissions

**Location:** `frontend/src/hooks/useGamePermissions.ts`

Get game permissions without requiring a GameContext. Useful for components that need game info but aren't wrapped in GameProvider.

```typescript
import { useGamePermissions } from '../hooks/useGamePermissions';

function MyComponent({ gameId }: { gameId: number }) {
  const {
    game,              // GameWithDetails | null
    participants,      // GameParticipant[]
    isLoading,         // boolean
    userRole,          // 'gm' | 'player' | 'co_gm' | 'audience' | 'none'
    isGM,              // boolean
    isCoGM,            // boolean
    isPlayer,          // boolean
    isAudience,        // boolean
    isParticipant,     // boolean (player or co_gm)
    canEditGame,       // boolean
    canManagePhases,   // boolean (GM or co-GM)
    canViewAllActions, // boolean (GM or co-GM)
    currentUserId,     // number | null
  } = useGamePermissions(gameId);

  return (
    <div>
      {isGM && <AdminPanel />}
      {canManagePhases && <PhaseControls />}
    </div>
  );
}
```

#### useUserCharacters

**Location:** `frontend/src/hooks/useUserCharacters.ts`

Fetch the current user's controllable characters for a game.

```typescript
import { useUserCharacters } from '../hooks/useUserCharacters';

function CharacterSelector({ gameId }: { gameId: number }) {
  const {
    characters,  // Character[]
    isLoading,   // boolean
    error,       // Error | null
    refetch      // () => Promise<void>
  } = useUserCharacters(gameId);

  if (isLoading) return <Spinner />;
  if (error) return <Error error={error} />;

  return (
    <select>
      {characters.map(char => (
        <option key={char.id} value={char.id}>
          {char.name}
        </option>
      ))}
    </select>
  );
}
```

#### useCharacterOwnership

**Location:** `frontend/src/hooks/useCharacterOwnership.ts`

Efficient character ownership checking using Set for O(1) lookup performance.

```typescript
import { useCharacterOwnership } from '../hooks/useCharacterOwnership';

function CharacterCard({
  gameId,
  characterId
}: {
  gameId: number;
  characterId: number;
}) {
  const {
    isUserCharacter,   // (characterId: number) => boolean
    userCharacterIds,  // Set<number>
    isLoading          // boolean
  } = useCharacterOwnership(gameId);

  const canEdit = isUserCharacter(characterId);

  return (
    <div>
      <h3>Character</h3>
      {canEdit && <button>Edit</button>}
    </div>
  );
}
```

---

## Usage Patterns

### Pattern 1: Full Context (Recommended for Game Pages)

Wrap entire game detail pages with GameProvider:

```typescript
function GameDetailsPage({ gameId }: { gameId: number }) {
  return (
    <GameProvider gameId={gameId}>
      <GameHeader />
      <GameTabs />
      <GameContent />
    </GameProvider>
  );
}

function GameContent() {
  const { game, isGM, userCharacters } = useGameContext();

  return (
    <div>
      {isGM && <AdminPanel />}
      <CharacterList chars={userCharacters} />
    </div>
  );
}
```

### Pattern 2: Hooks Without Context (Recommended for Smaller Components)

For components that need specific functionality without full context:

```typescript
function GameCard({ gameId }: { gameId: number }) {
  // No GameProvider needed!
  const { isGM, game } = useGamePermissions(gameId);
  const { characters } = useUserCharacters(gameId);

  return (
    <div>
      <h3>{game?.title}</h3>
      {isGM && <GMBadge />}
      <p>{characters.length} characters</p>
    </div>
  );
}
```

### Pattern 3: Mixed Approach (Use Context When Available)

Components that might be used both inside and outside a GameContext:

```typescript
function CharacterCard({ gameId, characterId }: Props) {
  // Try to use context first
  const gameContext = useOptionalGameContext();

  // Fall back to hook if no context
  const { isUserCharacter: hookChecker } = gameContext
    ? { isUserCharacter: gameContext.isUserCharacter }
    : useCharacterOwnership(gameId);

  const canEdit = hookChecker(characterId);

  return (
    <div>
      <h3>Character</h3>
      {canEdit && <EditButton />}
    </div>
  );
}
```

### Pattern 4: Role-Based Rendering

```typescript
function GameControls() {
  const { userRole, isGM } = useGameContext();

  return (
    <div>
      {isGM && <GMControls />}
      {userRole === 'player' && <PlayerControls />}
      {userRole === 'audience' && <AudienceView />}
    </div>
  );
}
```

### Pattern 5: Character Management

```typescript
function CharacterList() {
  const { userCharacters, isUserCharacter } = useGameContext();

  return (
    <div>
      {userCharacters.map(char => (
        <CharacterCard
          key={char.id}
          character={char}
          canEdit={true} // User owns all userCharacters
        />
      ))}
    </div>
  );
}
```

### Pattern 6: Permission-Based Actions

```typescript
function GameActions() {
  const { canEditGame, canManagePhases } = useGamePermissions(gameId);

  return (
    <div>
      {canEditGame && <button>Edit Game</button>}
      {canManagePhases && <button>Manage Phases</button>}
    </div>
  );
}
```

### Pattern 7: Loading States

```typescript
function MyComponent() {
  const { data: game, isLoading, error } = useGame(gameId);

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (!game) {
    return <NotFound />;
  }

  return <GameDisplay game={game} />;
}
```

### Pattern 8: Dependent Queries

```typescript
function CharacterSheet({ characterId }: Props) {
  // First query: Get character
  const { data: character } = useQuery({
    queryKey: ['characters', characterId],
    queryFn: () => apiClient.getCharacter(characterId),
  });

  // Second query: Get character's game (only runs if character exists)
  const { data: game } = useQuery({
    queryKey: ['games', character?.game_id],
    queryFn: () => apiClient.getGame(character!.game_id),
    enabled: !!character?.game_id, // Only run if we have game_id
  });

  return <div>...</div>;
}
```

### Pattern 9: Polling for Real-time Updates

```typescript
function useGamePhase(gameId: number) {
  return useQuery({
    queryKey: ['games', gameId, 'phase'],
    queryFn: () => apiClient.getCurrentPhase(gameId),
    refetchInterval: 10000, // Poll every 10 seconds
    refetchIntervalInBackground: true, // Continue polling in background
  });
}
```

---

## Migration Guide

### From Old useAuth Hook

**Before:**
```typescript
// Old hook from frontend/src/hooks/useAuth.ts
const { isAuthenticated, login, register, logout } = useAuth();
```

**After:**
```typescript
// New context-based approach
import { useAuth } from '../contexts/AuthContext';

const {
  currentUser,      // NEW: Access to user data
  isAuthenticated,
  login,
  register,
  logout
} = useAuth();

// Now you can access user ID immediately
const userId = currentUser?.id;
```

### From Manual User ID Fetching

**Before:**
```typescript
const [currentUserId, setCurrentUserId] = useState<number | null>(null);

useEffect(() => {
  const fetchCurrentUser = async () => {
    try {
      const response = await apiClient.getCurrentUser();
      setCurrentUserId(response.data.id);
    } catch (err) {
      console.error('Failed to fetch current user:', err);
    }
  };
  fetchCurrentUser();
}, []);
```

**After:**
```typescript
import { useAuth } from '../contexts/AuthContext';

const { currentUser } = useAuth();
const currentUserId = currentUser?.id;

// That's it! No manual fetching needed
```

### From Manual Permission Checks

**Before:**
```typescript
const [isGM, setIsGM] = useState(false);
const [isParticipant, setIsParticipant] = useState(false);

useEffect(() => {
  if (game && currentUserId) {
    setIsGM(game.gm_user_id === currentUserId);
    setIsParticipant(participants.some(p => p.user_id === currentUserId));
  }
}, [game, currentUserId, participants]);
```

**After (Option 1: Using GameContext):**
```typescript
// Wrap component in GameProvider
<GameProvider gameId={gameId}>
  <MyComponent />
</GameProvider>

// In MyComponent:
const { isGM, isParticipant } = useGameContext();
```

**After (Option 2: Using Hook):**
```typescript
const { isGM, isParticipant } = useGamePermissions(gameId);
```

### From Manual Character Ownership Checks

**Before:**
```typescript
const { data: controllableCharacters } = useQuery({
  queryKey: ['controllableCharacters', gameId],
  queryFn: () => apiClient.getUserControllableCharacters(gameId)
});

const isUserCharacter = (characterId: number) => {
  return controllableCharacters?.some(c => c.id === characterId) || false;
};
```

**After:**
```typescript
const { isUserCharacter } = useCharacterOwnership(gameId);

// Use it
if (isUserCharacter(characterId)) {
  // User owns this character
}
```

### Migration Checklist

1. Replace `useState` for user ID with `useAuth()`
2. Replace permission calculations with context/hooks
3. Remove duplicate React Query queries
4. Wrap game pages with `GameProvider`
5. Use hooks for smaller components
6. Test thoroughly after migration

### High-Priority Components to Migrate

1. `GameDetailsPage.tsx` - Most benefit from GameContext
2. `CommonRoom.tsx` - Uses user ID and characters
3. `CharactersList.tsx` - Uses ownership checks
4. `PrivateMessages.tsx` - Uses user ID and characters

---

## Best Practices

### 1. Use GameContext for Game-Focused Pages

Wrap entire game detail pages with GameProvider:

```typescript
function GameDetailsPage({ gameId }: { gameId: number }) {
  return (
    <GameProvider gameId={gameId}>
      <GameHeader />
      <GameTabs />
      <GameContent />
    </GameProvider>
  );
}
```

### 2. Use Hooks for Smaller Components

For components that need specific functionality without full context:

```typescript
function CharacterSelector({ gameId }: { gameId: number }) {
  // Lighter weight than GameContext
  const { characters } = useUserCharacters(gameId);

  return <select>{/* ... */}</select>;
}
```

### 3. Leverage React Query Caching

All hooks use React Query with 30-second stale time. Multiple components requesting the same data won't trigger duplicate API calls.

```typescript
// Component A
const { game } = useGamePermissions(gameId);

// Component B (same gameId)
const { game } = useGamePermissions(gameId);
// ↑ This uses cached data, no new API call!
```

### 4. Handle Loading States

Always check loading states before rendering:

```typescript
const { currentUser, isCheckingAuth } = useAuth();

if (isCheckingAuth) {
  return <Spinner />;
}

return <div>Welcome, {currentUser?.username}!</div>;
```

### 5. Use Optional Context for Reusable Components

Components that might be used both inside and outside a GameContext:

```typescript
function CharacterCard({ characterId }: { characterId: number }) {
  const gameContext = useOptionalGameContext();

  // Graceful fallback
  const canEdit = gameContext?.isUserCharacter(characterId) ?? false;

  return <div>{/* ... */}</div>;
}
```

### 6. Always Use isCheckingAuth

Prevent premature rendering by checking auth state:

```typescript
const { isCheckingAuth, isGM } = useAuth();

if (isCheckingAuth) {
  return <Spinner />;
}

return (
  <div>
    {!isGM && <button>Apply to Join</button>}
  </div>
);
```

---

## React Query Integration

### Query Configuration

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes
      retry: (failureCount, error) => {
        // Don't retry on auth errors
        if (error.status === 401) return false;
        return failureCount < 3;
      },
    },
    mutations: {
      retry: false, // Don't retry mutations by default
    },
  },
});
```

### Query Keys Used

```typescript
// Auth
['auth'] - Authentication state
['currentUser'] - Current user data

// Game
['gameDetails', gameId] - Game details
['gameParticipants', gameId] - Participants
['userControllableCharacters', gameId] - User's characters
['currentPhase', gameId] - Current phase
['gameCharacters', gameId] - All characters
```

### Custom Query Hooks

```typescript
export function useGame(gameId: number) {
  return useQuery({
    queryKey: ['games', gameId],
    queryFn: () => apiClient.getGame(gameId),
    enabled: !!gameId, // Only run if gameId exists
  });
}

export function useGames() {
  return useQuery({
    queryKey: ['games'],
    queryFn: () => apiClient.getAllGames(),
  });
}
```

### Mutation Hooks with Cache Invalidation

```typescript
export function useCreateGame() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateGameRequest) => apiClient.createGame(data),
    onSuccess: (newGame) => {
      // Invalidate and refetch games list
      queryClient.invalidateQueries({ queryKey: ['games'] });

      // Optionally set the new game in cache
      queryClient.setQueryData(['games', newGame.id], newGame);
    },
    onError: (error) => {
      console.error('Failed to create game:', error);
    },
  });
}
```

### Optimistic Updates

```typescript
export function useUpdateGame(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (updates: Partial<Game>) =>
      apiClient.updateGame(gameId, updates),

    // Optimistic update
    onMutate: async (updates) => {
      // Cancel outgoing queries
      await queryClient.cancelQueries({ queryKey: ['games', gameId] });

      // Save current value for rollback
      const previousGame = queryClient.getQueryData(['games', gameId]);

      // Optimistically update cache
      queryClient.setQueryData(['games', gameId], (old: Game) => ({
        ...old,
        ...updates,
      }));

      return { previousGame };
    },

    // Rollback on error
    onError: (error, updates, context) => {
      queryClient.setQueryData(['games', gameId], context.previousGame);
    },

    // Refetch on success
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games', gameId] });
      queryClient.invalidateQueries({ queryKey: ['games'] }); // Refresh list too
    },
  });
}
```

### Invalidate Queries

```typescript
import { useQueryClient } from '@tanstack/react-query';

const queryClient = useQueryClient();

// Invalidate game data
await queryClient.invalidateQueries({ queryKey: ['gameDetails', gameId] });

// Invalidate user characters
await queryClient.invalidateQueries({ queryKey: ['userControllableCharacters', gameId] });

// Invalidate current user
await queryClient.invalidateQueries({ queryKey: ['currentUser'] });
```

### Cache Strategy

All queries use appropriate stale times:

```typescript
// User data: 5 minutes (changes infrequently)
staleTime: 5 * 60 * 1000

// Game data: 30 seconds (may change during play)
staleTime: 30000
```

**Rationale:**
- User data doesn't change frequently
- Game data may change during active play
- Reduces unnecessary API calls
- Users get immediate feedback from cache
- Can manually refetch when needed

---

## Performance

### Before Implementation

- Multiple components fetching same user data independently
- Permission checks recalculated in every component
- No caching of game data
- Duplicate API calls

### After Implementation

- Single user data fetch per login, cached
- Permission checks calculated once, shared via context
- React Query caches all game data
- No duplicate API calls (verified via Network tab)

### Benchmarks

**API Call Reduction:**
- User data: 3-5 calls per page → 1 call per session
- Game details: 2-3 calls per page → 1 call per 30 seconds
- Participants: 2-3 calls per page → 1 call per 30 seconds
- Characters: 2-3 calls per page → 1 call per 30 seconds

**Estimated Improvement:** 60-70% reduction in API calls

### Performance Characteristics

1. **React Query Caching**: All data is cached for 30 seconds, reducing API calls
2. **Memoization**: Permission flags and checkers are memoized
3. **Conditional Fetching**: Queries only run when conditions are met (e.g., user is authenticated)
4. **Granular Updates**: Only affected components re-render when data changes
5. **Set-based Lookups**: Character ownership uses Set for O(1) performance

### Cache Flow

```
Component A requests game details
    │
    ├──> Check cache: ['gameDetails', 123]
    │       │
    │       ├──> Cache MISS → Fetch from API
    │       │       │
    │       │       └──> Store in cache (staleTime: 30s)
    │       │
    │       └──> Cache HIT → Return cached data
    │
    └──> Component A receives data


Component B requests same game details (within 30s)
    │
    ├──> Check cache: ['gameDetails', 123]
    │       │
    │       └──> Cache HIT → Return cached data instantly
    │                        (NO API call!)
    │
    └──> Component B receives data


After 30 seconds (data is stale)
    │
    ├──> Background refetch triggered
    │       │
    │       └──> API: GET /api/v1/games/123/details
    │               │
    │               └──> Update cache with fresh data
    │
    └──> All components re-render with new data
```

---

## Testing

### Testing Components with AuthContext

```typescript
import { render, screen } from '@testing-library/react';
import { AuthContext } from '../contexts/AuthContext';

const mockAuthValue = {
  currentUser: { id: 1, username: 'testuser' },
  isAuthenticated: true,
  isLoading: false,
  isCheckingAuth: false,
  login: jest.fn(),
  register: jest.fn(),
  logout: jest.fn(),
  error: null,
};

test('renders with authenticated user', () => {
  render(
    <AuthContext.Provider value={mockAuthValue}>
      <MyComponent />
    </AuthContext.Provider>
  );
  expect(screen.getByText('Welcome, testuser!')).toBeInTheDocument();
});
```

### Testing Components with React Query

```typescript
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
};

test('loads and displays game', async () => {
  render(<GameDetails gameId={1} />, { wrapper: createWrapper() });
  expect(await screen.findByText('Game Title')).toBeInTheDocument();
});
```

### Testing Recommendations

**Unit Tests:**
- Test that AuthContext provides correct user data
- Test that GameContext calculates permissions correctly
- Test character ownership checker logic
- Test hook return values

**Integration Tests:**
- Test that login populates currentUser
- Test that GameProvider fetches all required data
- Test that permission flags update when role changes
- Test that refetch functions work correctly

**Manual Testing:**
- Verify user ID appears after login
- Check that GM controls only show for GMs
- Verify character ownership badges appear correctly
- Test that data refreshes appropriately

---

## Debugging

### Debug Logging

All contexts and hooks include debug logging:

```typescript
// In browser console:
// [AuthContext] Context state: { isAuthenticated: true, hasUser: true, userId: 123 }
// [GameContext] Context state: { gameId: 1, userRole: 'gm', isGM: true }
// [useUserCharacters] Characters loaded: [...]
```

Enable these logs to troubleshoot state issues.

### React Query DevTools

```typescript
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';

<QueryClientProvider client={queryClient}>
  <AuthProvider>
    {/* App */}
  </AuthProvider>
  <ReactQueryDevtools initialIsOpen={false} />
</QueryClientProvider>
```

### Common Issues

**Issue:** `currentUser` is null after login
- **Fix:** Ensure AuthProvider wraps app, check token storage

**Issue:** GameContext throws error
- **Fix:** Wrap component with GameProvider or use hook instead

**Issue:** Duplicate API calls
- **Fix:** Check React Query DevTools, remove competing queries

**Issue:** Permissions not updating
- **Fix:** Call `refetchGameData()` after mutations

---

## TypeScript Support

All contexts and hooks are fully typed:

```typescript
interface AuthContextValue {
  currentUser: User | null;
  isAuthenticated: boolean;
  // ...
}

type UserGameRole = 'gm' | 'player' | 'co_gm' | 'audience' | 'none';

interface GamePermissions {
  userRole: UserGameRole;
  isGM: boolean;
  // ...
}
```

### Common Types

```typescript
import type { User } from '../types/auth';
import type { GameWithDetails, GameParticipant } from '../types/games';
import type { Character } from '../types/characters';
import type { UserGameRole } from '../contexts/GameContext';

// User
const user: User = {
  id: 1,
  username: 'player1',
  email: 'player1@example.com'
};

// Role
const role: UserGameRole = 'gm' | 'player' | 'co_gm' | 'audience' | 'none';
```

Use TypeScript's autocomplete to explore available properties.

---

## Quick Reference

### Import Statements

```typescript
// Enhanced Auth Context
import { useAuth } from '../contexts/AuthContext';

// Game Context
import { GameProvider, useGameContext, useOptionalGameContext } from '../contexts/GameContext';

// Custom Hooks
import { useGamePermissions } from '../hooks/useGamePermissions';
import { useUserCharacters } from '../hooks/useUserCharacters';
import { useCharacterOwnership } from '../hooks/useCharacterOwnership';
```

### Get Current User

```typescript
const { currentUser, isAuthenticated } = useAuth();

// Access user properties
const userId = currentUser?.id;
const username = currentUser?.username;
const email = currentUser?.email;
```

### Check User Role in Game

```typescript
// Option 1: Using GameContext (inside GameProvider)
const { userRole, isGM, isParticipant } = useGameContext();

// Option 2: Using hook (anywhere)
const { userRole, isGM, isParticipant } = useGamePermissions(gameId);
```

### Get User's Characters

```typescript
// Option 1: Using GameContext
const { userCharacters } = useGameContext();

// Option 2: Using hook
const { characters } = useUserCharacters(gameId);
```

### Check Character Ownership

```typescript
// Option 1: Using GameContext
const { isUserCharacter } = useGameContext();
if (isUserCharacter(characterId)) {
  // User owns this character
}

// Option 2: Using hook
const { isUserCharacter } = useCharacterOwnership(gameId);
if (isUserCharacter(characterId)) {
  // User owns this character
}
```

### When to Use What

| Use Case | Recommended Approach |
|----------|---------------------|
| Get current user anywhere | `useAuth()` |
| Game detail page | Wrap with `GameProvider` |
| Check game permissions | `useGamePermissions(gameId)` |
| List user's characters | `useUserCharacters(gameId)` |
| Check character ownership | `useCharacterOwnership(gameId)` |
| Reusable component (may be outside game) | `useOptionalGameContext()` |
| Small widget needing game info | Use specific hook |
| Complex page with multiple game queries | Wrap with `GameProvider` |

---

## State Management Anti-Patterns

### ❌ Don't Decode JWT Client-Side

```typescript
// ❌ WRONG: Security risk
const token = localStorage.getItem('access_token');
const decoded = JSON.parse(atob(token.split('.')[1]));
const userId = decoded.user_id;

// ✅ CORRECT: Use AuthContext
const { currentUser } = useAuth();
const userId = currentUser?.id;
```

### ❌ Don't Fetch User Data in Multiple Places

```typescript
// ❌ WRONG: Duplicate API calls
export function MyComponent() {
  const [user, setUser] = useState(null);
  useEffect(() => {
    apiClient.getCurrentUser().then(setUser);
  }, []);
  // ...
}

// ✅ CORRECT: Use centralized AuthContext
export function MyComponent() {
  const { currentUser } = useAuth();
  // ...
}
```

### ❌ Don't Forget isCheckingAuth

```typescript
// ❌ WRONG: Race condition
{!isGM && <button>Apply</button>}

// ✅ CORRECT: Wait for auth
{!isGM && !isCheckingAuth && <button>Apply</button>}
```

### ❌ Don't Store Server Data in useState

```typescript
// ❌ WRONG: Manual state management
const [games, setGames] = useState([]);
useEffect(() => {
  apiClient.getGames().then(setGames);
}, []);

// ✅ CORRECT: Use React Query
const { data: games } = useQuery({
  queryKey: ['games'],
  queryFn: () => apiClient.getGames(),
});
```

---

## Related Files

### Core Implementation
- `frontend/src/contexts/AuthContext.tsx` - Enhanced auth context
- `frontend/src/contexts/GameContext.tsx` - Game context
- `frontend/src/hooks/useGamePermissions.ts` - Permissions hook
- `frontend/src/hooks/useUserCharacters.ts` - User characters hook
- `frontend/src/hooks/useCharacterOwnership.ts` - Ownership checker hook
- `frontend/src/types/auth.ts` - Auth type definitions

### Documentation
- `.claude/context/STATE_MANAGEMENT.md` - AI context (pointer to this doc)
- `/docs/adrs/005-frontend-state-management.md` - ADR with architectural decisions

### Examples
- See updated `GameDetailsPage.tsx` for GameContext usage
- See updated `CommonRoom.tsx` for AuthContext usage

---

## Summary

The unified state management system provides:

✅ Centralized user identity management
✅ Consolidated game permissions and role logic
✅ Efficient character ownership checking
✅ Reduced API calls via React Query caching
✅ Improved developer experience with TypeScript
✅ Comprehensive documentation and migration guide
✅ Backward compatibility with existing code

**Result:** Cleaner, more maintainable code with better performance and developer experience.
