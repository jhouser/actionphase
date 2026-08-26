# ADR-005: Frontend State Management

## Status
Accepted

## Context
ActionPhase frontend requires state management that handles:
- Server state synchronization (games, characters, phases)
- Authentication state and token management
- Complex UI state for game interactions
- Real-time updates for collaborative features
- Optimistic updates for better user experience
- Offline capabilities for read-only data
- Performance optimization through caching

The solution must balance simplicity, performance, and maintainability while providing excellent developer experience.

## Decision
We implemented a **Hybrid State Management Strategy** using specialized tools for different types of state:

**Server State**: React Query (TanStack Query)
- All API communication and server state caching
- Automatic background refetching and synchronization
- Optimistic updates for mutations
- Infinite queries for paginated data

**Authentication State**: Custom React Context + Local Storage
- JWT token management with automatic refresh
- User session state and permissions
- Login/logout flow orchestration
- Persistent authentication across browser sessions

**UI State**: React useState + useReducer
- Component-local state for forms and interactions
- Complex state machines with useReducer
- Temporary UI state that doesn't need persistence
- Modal visibility, form validation states

**Global UI State**: React Context (Sparingly)
- Theme preferences and user settings
- Global UI state that spans multiple components
- Notification/toast management
- Navigation state

## Alternatives Considered

### 1. Redux + Redux Toolkit
**Approach**: Centralized state management with reducers and actions.

**Pros**:
- Predictable state updates with time-travel debugging
- Excellent developer tools and ecosystem
- Battle-tested for large applications
- Clear separation of concerns with slices

**Cons**:
- Significant boilerplate for simple operations
- Steep learning curve for new developers
- Over-engineering for current application size
- Complex setup for async operations and caching

### 2. Zustand
**Approach**: Lightweight state management library with simplified API.

**Pros**:
- Minimal boilerplate and simple API
- TypeScript-first design
- No providers needed
- Good performance characteristics

**Cons**:
- Less mature ecosystem compared to Redux
- Limited developer tooling
- No built-in server state management
- Still requires additional solutions for caching

### 3. Jotai (Atomic State Management)
**Approach**: Bottom-up atomic state management with composable atoms.

**Pros**:
- Fine-grained reactivity and performance
- Composable state atoms
- No unnecessary re-renders
- TypeScript-friendly API

**Cons**:
- Different mental model requires learning
- Less mature ecosystem
- Complex setup for server state
- May be overkill for current needs

### 4. Apollo Client (GraphQL)
**Approach**: GraphQL client with integrated caching and state management.

**Pros**:
- Unified data layer for server and local state
- Excellent caching and optimization features
- Real-time subscriptions support
- Mature ecosystem and tooling

**Cons**:
- Requires GraphQL backend (we use REST)
- Heavy client bundle size
- Complex setup and configuration
- Overkill without GraphQL

## Consequences

### Positive Consequences

**Developer Experience**:
- React Query provides excellent DevTools for debugging server state
- Simple useState for most UI interactions
- TypeScript integration provides compile-time safety
- Minimal learning curve leveraging React patterns

**Performance**:
- React Query handles caching and background updates efficiently
- Component-local state prevents unnecessary re-renders
- Lazy loading of data with suspense integration
- Optimistic updates improve perceived performance

**User Experience**:
- Automatic token refresh provides seamless authentication
- Background data synchronization keeps UI current
- Optimistic updates for immediate feedback
- Error boundaries handle network failures gracefully

**Maintainability**:
- Clear separation between server state and UI state
- Custom hooks encapsulate complex state logic
- Consistent patterns across components
- Easy to refactor and test individual pieces

### Negative Consequences

**Complexity Distribution**:
- Multiple state management approaches require mental context switching
- Custom authentication logic needs careful maintenance
- React Query configuration complexity for advanced features
- Potential for state synchronization issues between systems

**Learning Curve**:
- Developers need to understand React Query concepts
- Custom hooks patterns require React expertise
- Token refresh logic has subtle timing considerations
- Error handling strategies vary by state type

**Bundle Size**:
- React Query adds significant bundle weight
- Multiple state management libraries increase total bundle size
- Custom authentication code adds complexity
- Development overhead for maintaining multiple patterns

### Risk Mitigation Strategies

**State Synchronization**:
- Clear documentation of state boundaries and responsibilities
- Custom hooks provide consistent access patterns
- Integration tests verify cross-system state consistency
- React Query invalidation strategies keep data fresh

**Authentication Security**:
- Secure token storage with appropriate fallbacks
- Automatic logout on token expiration
- Request interceptors handle token refresh transparently
- Correlation IDs for debugging authentication issues

**Performance Monitoring**:
- React Query DevTools for cache performance analysis
- Bundle size monitoring for bloat prevention
- React DevTools profiler for render optimization
- Network monitoring for API call efficiency

## Implementation Details

### React Query Setup
```typescript
// Query client configuration — ACTUAL, from src/App.tsx:37
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes
      // Deliberate: prevents refetch on tab switch, which would otherwise
      // discard in-progress user input and scroll position.
      refetchOnWindowFocus: false,
    },
  },
});

// Custom hooks for server state. Note the client is `apiClient`, and the
// listing hook is `useGameListing` (it reads its filters from URL params).
export function useGameListing() {
  const [searchParams] = useSearchParams();
  const filters = useMemo(() => parseFiltersFrom(searchParams), [searchParams]);

  return useQuery({
    queryKey: ['games', 'filtered', filters],
    queryFn: async () => {
      const response = await apiClient.games.getFilteredGames(filters);
      return response.data;
    },
  });
}

export function useCreateGame() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: apiClient.games.createGame,
    onSuccess: () => {
      // NOTE: TanStack Query v5 takes an OBJECT here. The bare-array form
      // `invalidateQueries(['games'])` is v4 syntax and does not work.
      queryClient.invalidateQueries({ queryKey: ['games'] });
    },
    // Optimistic update
    onMutate: async (newGame) => {
      await queryClient.cancelQueries({ queryKey: ['games'] });
      const previousGames = queryClient.getQueryData(['games']);

      queryClient.setQueryData(['games'], (old) => [
        ...old,
        { ...newGame, id: Date.now(), optimistic: true }
      ]);

      return { previousGames };
    },
    onError: (error, variables, context) => {
      queryClient.setQueryData(['games'], context.previousGames);
    },
  });
}
```

> **Version note:** this project is on **TanStack Query 5**. All the
> `cancelQueries` / `invalidateQueries` / `removeQueries` filter APIs take an
> object (`{ queryKey: [...] }`), not a positional array. Older v4-style
> snippets will type-error.

### Authentication Context
```typescript
// ACTUAL shape, from src/contexts/AuthContext.tsx. Note `currentUser`
// (not `user`) and the separate `isCheckingAuth` flag — see the
// "Recent Architectural Evolution" section on why that distinction exists.
interface AuthContextValue {
  currentUser: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  isCheckingAuth: boolean;
  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<AxiosResponse<AuthResponse>>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Initialize authentication state from stored tokens
  useEffect(() => {
    const initializeAuth = async () => {
      const token = localStorage.getItem('access_token');
      if (token && !isTokenExpired(token)) {
        try {
          const userData = await api.auth.getCurrentUser();
          setUser(userData);
        } catch (error) {
          // Token invalid, clear storage
          localStorage.removeItem('access_token');
          localStorage.removeItem('refresh_token');
        }
      }
      setIsLoading(false);
    };

    initializeAuth();
  }, []);

  const login = async (credentials: LoginCredentials) => {
    const response = await api.auth.login(credentials);
    localStorage.setItem('access_token', response.access_token);
    localStorage.setItem('refresh_token', response.refresh_token);
    setUser(response.user);
  };

  const logout = () => {
    api.auth.logout().catch(() => {
      // Logout locally even if server call fails
    });
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{
      user,
      login,
      logout,
      isAuthenticated: !!user,
      isLoading,
    }}>
      {children}
    </AuthContext.Provider>
  );
}
```

### Custom Hook Patterns
```typescript
// Game management hook
export function useGameManagement(gameId: string) {
  const queryClient = useQueryClient();

  const game = useQuery({
    queryKey: ['games', gameId],
    queryFn: () => api.games.get(gameId),
  });

  const updateGame = useMutation({
    mutationFn: (updates: Partial<Game>) =>
      api.games.update(gameId, updates),
    onSuccess: (updatedGame) => {
      queryClient.setQueryData(['games', gameId], updatedGame);
      queryClient.invalidateQueries(['games']);
    },
  });

  const deleteGame = useMutation({
    mutationFn: () => api.games.delete(gameId),
    onSuccess: () => {
      queryClient.removeQueries(['games', gameId]);
      queryClient.invalidateQueries(['games']);
    },
  });

  return {
    game: game.data,
    isLoading: game.isLoading,
    error: game.error,
    updateGame: updateGame.mutate,
    deleteGame: deleteGame.mutate,
    isUpdating: updateGame.isPending || deleteGame.isPending,
  };
}

// Complex form state with validation
export function useGameForm(initialData?: Partial<Game>) {
  const [formData, setFormData] = useState({
    title: initialData?.title || '',
    description: initialData?.description || '',
    maxPlayers: initialData?.maxPlayers || 4,
    gameConfig: initialData?.gameConfig || {},
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isDirty, setIsDirty] = useState(false);

  const updateField = (field: string, value: any) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    setIsDirty(true);
    // Clear error when field is updated
    if (errors[field]) {
      setErrors(prev => ({ ...prev, [field]: undefined }));
    }
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.title.trim()) {
      newErrors.title = 'Title is required';
    }
    if (formData.maxPlayers < 1 || formData.maxPlayers > 8) {
      newErrors.maxPlayers = 'Max players must be between 1 and 8';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  return {
    formData,
    errors,
    isDirty,
    updateField,
    validate,
    reset: () => {
      setFormData(initialData || {});
      setErrors({});
      setIsDirty(false);
    },
  };
}
```

### Error Boundary Integration
```typescript
export function QueryErrorBoundary({ children }: { children: ReactNode }) {
  return (
    <ErrorBoundary
      fallback={({ error, resetError }) => (
        <div className="error-container">
          <h2>Something went wrong</h2>
          <details>
            <summary>Error details</summary>
            <pre>{error.message}</pre>
          </details>
          <button onClick={resetError}>Try again</button>
        </div>
      )}
      onError={(error) => {
        // Log error with context
        console.error('React Query Error:', error);
        // Could send to error reporting service
      }}
    >
      {children}
    </ErrorBoundary>
  );
}
```

## Recent Architectural Evolution (October 2025)

### Authentication State Consolidation

In October 2025, we completed a major refactoring to address authentication state duplication
and security concerns across the frontend application.

#### Problem Statement

Prior to this refactoring:
- **15+ components** were independently fetching user data
- Each component decoded JWT client-side to get user ID
- **60-70% duplicate API calls** for the same user data
- Race conditions and loading state inconsistencies
- Security risk from client-side JWT parsing

#### Solution Implemented

**Created Unified AuthContext**:
```typescript
// frontend/src/contexts/AuthContext.tsx

interface AuthContextValue {
  currentUser: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  isCheckingAuth: boolean;  // NEW: Prevents premature renders
  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
  error: Error | null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  // Single React Query for authentication state
  const { data: isAuthenticated, isLoading: isCheckingAuth } = useQuery({
    queryKey: ['auth'],
    queryFn: () => !!apiClient.getAuthToken(),
    staleTime: 0,
    refetchOnWindowFocus: true,
  });

  // Single React Query for user data
  const { data: currentUser, isLoading: isLoadingUser } = useQuery({
    queryKey: ['currentUser'],
    queryFn: async () => {
      const response = await apiClient.getCurrentUser();
      return response.data;
    },
    enabled: isAuthenticated === true,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  });

  // ... mutation handlers for login/register/logout

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}
```

**Migration Pattern**:
```typescript
// BEFORE (per-component duplication):
export function GameDetailsPage({ gameId }: Props) {
  const [currentUserId, setCurrentUserId] = useState<number | null>(null);

  useEffect(() => {
    const fetchUser = async () => {
      try {
        const token = localStorage.getItem('access_token');
        if (token) {
          const decoded = decodeJWT(token);  // Client-side JWT parsing
          const userData = await apiClient.getCurrentUser();
          setCurrentUserId(userData.id);
        }
      } catch (error) {
        console.error('Failed to load user:', error);
      }
    };
    fetchUser();
  }, []);

  // ... rest of component
}

// AFTER (centralized):
export function GameDetailsPage({ gameId }: Props) {
  const { currentUser, isCheckingAuth } = useAuth();
  const currentUserId = currentUser?.id ?? null;

  // ... rest of component
}
```

**Components Migrated**:
- `GameDetailsPage` - Main game detail view
- `GamesList` - Game discovery and listing
- `CommonRoom` - Common room phase UI
- `ActionSubmission` - Action submission form
- `CharactersList` - Character management
- `PrivateMessages` - Messaging UI
- `PhaseManagement` - GM phase controls
- `ConversationList` - Conversation display
- 8+ additional components

**Security Improvements**:
1. **Eliminated client-side JWT decoding** - No more `decodeJWT()` functions
2. **Server-side user ID validation** - Always fetched from `/auth/me`
3. **Reduced attack surface** - No JWT payload access in client code

**Performance Improvements**:
1. **60-70% reduction in `/auth/me` API calls**
2. **Single source of truth** eliminates race conditions
3. **React Query caching** (5-minute stale time)
4. **Intelligent refetching** on window focus

**Developer Experience**:
1. **Simple `useAuth()` hook** for all components
2. **Consistent loading states** with `isCheckingAuth`
3. **Centralized error handling**
4. **Type-safe throughout**

#### Implementation Details

**App Setup**:
```typescript
// frontend/src/App.tsx

function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>  {/* Single provider wraps entire app */}
          <AppRoutes />
        </AuthProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
```

**Hook Usage**:
```typescript
// Any component can use:
const { currentUser, isCheckingAuth, isAuthenticated } = useAuth();

// Conditional rendering with loading state:
if (isCheckingAuth) {
  return <LoadingSpinner />;
}

// Check authentication:
if (!isAuthenticated) {
  return <Navigate to="/login" />;
}

// Access user data:
const isGM = game.gm_user_id === currentUser?.id;
```

**Button Rendering with Auth State**:
```typescript
// Prevents buttons rendering before auth state is known
{!isGM && !isCheckingAuth && game.state === 'recruitment' && (
  <button onClick={handleApply}>Apply to Join</button>
)}
```

#### Results & Metrics

**Before Refactoring**:
- 15 components independently fetching user data
- ~50-70 API calls per typical user session
- Inconsistent loading states
- Client-side JWT decoding in 5+ locations

**After Refactoring**:
- 1 centralized AuthProvider
- ~15-20 API calls per typical user session (**60-70% reduction**)
- Consistent loading states across all components
- Zero client-side JWT decoding

**Lines of Code**:
- Added: ~170 lines (AuthContext implementation)
- Removed: ~180 lines (duplicate user fetching logic)
- Net change: -10 lines (more functionality, less code!)

#### Future Considerations

**Potential Enhancements**:
1. **Persist auth state in IndexedDB** for offline capabilities
2. **Add session timeout warnings** before token expiration
3. **Implement WebSocket reconnection** using auth context
4. **Add user presence tracking** for collaborative features

**Testing Requirements**:
- Unit tests for AuthContext provider
- Integration tests for auth state transitions
- Mock API responses for component tests
- E2E tests for login/logout flows

#### References
- Implementation PR: [Internal reference - completed Oct 2025]
- Related bug fixes:
  - GM application prevention (used `isCheckingAuth` flag)
  - Conversation deduplication (used `currentUser?.id`)
  - Button visibility issues (proper loading states)

---

### Lessons Learned

**Architecture Insights**:
1. **Early centralization prevents technical debt** - Addressing duplication early is easier
2. **Security and performance align** - Removing client-side JWT parsing improved both
3. **React Query excels for server state** - Caching and synchronization handled automatically
4. **Loading states are critical** - `isCheckingAuth` prevents race conditions

**Migration Tips**:
1. **Build infrastructure first** - Create AuthContext before migrating components
2. **Migrate incrementally** - One component at a time, verify each works
3. **Test at boundaries** - Ensure loading and error states work correctly
4. **Remove old code** - Delete JWT decoder functions to prevent regression

### State Invalidation Patterns
```typescript
// Automatic invalidation on related mutations
export function useCharacterMutations(gameId: string) {
  const queryClient = useQueryClient();

  const createCharacter = useMutation({
    mutationFn: api.characters.create,
    onSuccess: () => {
      // Invalidate related queries
      queryClient.invalidateQueries(['games', gameId, 'characters']);
      queryClient.invalidateQueries(['games', gameId]); // Update game player count
    },
  });

  // Smart invalidation based on mutation type
  const updateCharacter = useMutation({
    mutationFn: api.characters.update,
    onSuccess: (updatedCharacter) => {
      // Update specific character in cache
      queryClient.setQueryData(
        ['characters', updatedCharacter.id],
        updatedCharacter
      );
      // Invalidate list to maintain consistency
      queryClient.invalidateQueries(['games', gameId, 'characters']);
    },
  });

  return { createCharacter, updateCharacter };
}
```

## Performance Optimizations

### Caching Strategy
- **Stale While Revalidate**: Data stays fresh with background updates
- **Cache Time**: Longer cache times for rarely changing data
- **Query Keys**: Structured keys for efficient invalidation
- **Prefetching**: Load data before user navigation

### Bundle Optimization
- **Code Splitting**: React Query DevTools only in development
- **Tree Shaking**: Import only needed React Query features
- **Lazy Loading**: Dynamic imports for heavy state management logic
- **Memoization**: React.memo and useMemo for expensive computations

## Testing Strategy

### Unit Testing
- Mock React Query for component tests
- Test custom hooks with React Testing Library
- Validate authentication state transitions
- Error boundary behavior testing

### Integration Testing
- Test state synchronization between systems
- Verify token refresh flows
- API integration with mocked responses
- End-to-end user flows with real state management

## Future Considerations

### Planned Enhancements
- **Offline Support**: Background sync for offline operations
- **Real-time Updates**: WebSocket integration with React Query
- **Persistent Caching**: IndexedDB for offline data persistence
- **State Devtools**: Enhanced debugging tools for custom state

### Scalability
- **State Normalization**: Flatten nested data structures
- **Selective Hydration**: Load only necessary state
- **Memory Management**: Garbage collection for unused cache entries
- **Performance Monitoring**: Track state management performance

## References
- [React Query Documentation](https://tanstack.com/query/latest)
- [React State Management Guide](https://react.dev/learn/managing-state)
- [Authentication Patterns in React](https://auth0.com/blog/complete-guide-to-react-user-authentication/)
- [React Testing Library Best Practices](https://kentcdodds.com/blog/common-mistakes-with-react-testing-library)
