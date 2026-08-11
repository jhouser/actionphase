package http

import (
	"actionphase/pkg/admin"
	"actionphase/pkg/auth"
	"actionphase/pkg/avatars"
	"actionphase/pkg/characters"
	"actionphase/pkg/conversations"
	"actionphase/pkg/core"
	"actionphase/pkg/dashboard"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	dbphases "actionphase/pkg/db/services/phases"
	"actionphase/pkg/deadlines"
	"actionphase/pkg/docs"
	"actionphase/pkg/exports"
	"actionphase/pkg/games"
	"actionphase/pkg/handouts"
	httpmiddleware "actionphase/pkg/http/middleware"
	"actionphase/pkg/messages"
	ratelimitmw "actionphase/pkg/middleware"
	"actionphase/pkg/notifications"
	"actionphase/pkg/phases"
	"actionphase/pkg/polls"
	"actionphase/pkg/users"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Handler struct {
	App *core.App
}

// getTokenAuth creates JWT auth using the app configuration
func (h *Handler) getTokenAuth() *jwtauth.JWTAuth {
	return jwtauth.New(h.App.Config.JWT.Algorithm, []byte(h.App.Config.JWT.Secret), nil)
}

// sessionValidateMW returns middleware that rejects requests whose JWT token no longer
// has a matching row in the sessions table (e.g. after an IP/fingerprint ban or logout).
func (h *Handler) sessionValidateMW() func(http.Handler) http.Handler {
	sessionSvc := &db.SessionService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	return core.ValidateSessionMiddleware(sessionSvc)
}

func (h *Handler) Start() {
	r := chi.NewRouter()

	// Add observability middleware stack first
	observabilityMiddleware := h.App.Observability.MiddlewareStack()
	for _, mw := range observabilityMiddleware {
		r.Use(mw)
	}

	// Keep existing middleware for compatibility
	r.Use(middleware.RequestID)
	r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root."))
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ponger"))
	})

	// Observability endpoints
	r.Get("/health", h.App.Observability.HealthHandler())

	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	})

	apiV1Router := chi.NewRouter()

	authRouter := chi.NewRouter()
	authRouter.Route("/", func(r chi.Router) {
		authHandler := auth.Handler{
			App:                    h.App,
			UserService:            &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			SessionService:         &db.SessionService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			UserPreferencesService: db.NewUserPreferencesService(h.App.Pool),
			IPBanService:           &db.IPBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			FingerprintBanService:  &db.FingerprintBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			DiscordService:         &db.DiscordAccountService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}

		// Public routes (no authentication required)
		// Apply strict rate limiting to sensitive endpoints
		// In development mode, rate limiting is relaxed for E2E testing
		isDev := h.App.Config.IsDevelopment()
		r.With(ratelimitmw.StrictRateLimit(isDev)).Post("/register", authHandler.V1Register)
		r.With(ratelimitmw.StrictRateLimit(isDev)).Post("/login", authHandler.V1Login)
		r.Post("/logout", authHandler.V1Logout) // Logout endpoint (clears JWT cookie)
		r.With(ratelimitmw.StrictRateLimit(isDev)).Post("/request-password-reset", authHandler.V1RequestPasswordReset)
		r.Post("/reset-password", authHandler.V1ResetPassword)
		r.Get("/validate-reset-token", authHandler.V1ValidateResetToken)
		r.Post("/verify-email", authHandler.V1VerifyEmail)                  // Verify email with token
		r.Post("/complete-email-change", authHandler.V1CompleteEmailChange) // Complete email change with token

		// Probe endpoint: returns current user if authenticated, null if not (no 401)
		r.With(jwtauth.Verifier(h.getTokenAuth())).Get("/me", authHandler.V1Me)

		// Discord OAuth callback (public — Discord redirects here after authorization)
		r.Get("/discord/callback", authHandler.V1DiscordCallback)

		// Protected routes (require authentication)
		r.Group(func(r chi.Router) {
			// Seek, verify and validate JWT tokens
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Get("/refresh", authHandler.V1Refresh)
			r.Get("/preferences", authHandler.V1GetPreferences)    // Get user preferences
			r.Put("/preferences", authHandler.V1UpdatePreferences) // Update user preferences
			r.Get("/users/search", authHandler.V1SearchUsers)      // Search for users
			// Discord OAuth routes (protected)
			r.Get("/discord/connect", authHandler.V1DiscordConnect)                                                        // Get Discord OAuth URL
			r.Get("/discord/status", authHandler.V1DiscordStatus)                                                          // Check Discord link status
			r.Delete("/discord/disconnect", authHandler.V1DiscordDisconnect)                                               // Unlink Discord account
			r.Post("/change-password", authHandler.V1ChangePassword)                                                       // Change password (authenticated users)
			r.With(ratelimitmw.StrictRateLimit(isDev)).Post("/resend-verification", authHandler.V1ResendVerificationEmail) // Resend email verification with rate limiting
			r.Post("/change-username", authHandler.V1ChangeUsername)                                                       // Change username
			r.Post("/request-email-change", authHandler.V1RequestEmailChange)                                              // Request email change
			r.Delete("/account", authHandler.V1DeleteAccount)                                                              // Soft delete account
			r.Get("/sessions", authHandler.V1ListSessions)                                                                 // List active sessions
			r.Delete("/sessions/{sessionID}", authHandler.V1RevokeSession)                                                 // Revoke specific session
			r.Post("/revoke-all-sessions", authHandler.V1RevokeAllSessions)                                                // Revoke all sessions except current
		})
	})
	apiV1Router.Mount("/auth", authRouter)

	// Games API - All routes require authentication
	gamesRouter := chi.NewRouter()
	gamesRouter.Route("/", func(r chi.Router) {
		gameHandler := games.Handler{
			App:                     h.App,
			UserService:             &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:             &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameApplicationService:  &db.GameApplicationService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			CharacterService:        &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			NotificationService:     db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
			MessageService:          &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			ActionSubmissionService: &dbactions.ActionSubmissionService{DB: h.App.Pool, Logger: h.App.ObsLogger, NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger)},
			GameStatsService:        &db.GameStatsService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}
		userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}

		// Public routes (authentication optional - will enrich if present)
		tokenAuth := h.getTokenAuth()
		r.Group(func(r chi.Router) {
			// Use verifier to extract token if present, but don't require authentication
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Get("/", gameHandler.GetFilteredGames)                                                              // Main game listing endpoint with filters
			r.With(gameHandler.GameMiddleware()).Get("/{gameID}/applicants", gameHandler.GetPublicGameApplicants) // Public list of applicants (username + role only)
		})

		// All routes below require authentication
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// Game listing and viewing
			r.Get("/recruiting", gameHandler.GetRecruitingGames)

			// Create game requires email verification
			r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).Post("/", gameHandler.CreateGame)
			r.Route("/{gameID}", func(r chi.Router) {
				r.Use(gameHandler.GameMiddleware())

				r.Get("/", gameHandler.GetGame)
				r.Get("/details", gameHandler.GetGameWithDetails)
				r.Get("/participants", gameHandler.GetGameParticipants)

				// Game management
				r.Put("/", gameHandler.UpdateGame)
				r.Delete("/", gameHandler.DeleteGame)
				r.Put("/state", gameHandler.UpdateGameState)
				r.Post("/banner", gameHandler.UploadGameBanner)
				r.Delete("/banner", gameHandler.DeleteGameBanner)

				// Participant management
				r.Delete("/leave", gameHandler.LeaveGame)
				r.Delete("/participants/{userId}", gameHandler.RemovePlayer)           // GM removes player
				r.Post("/participants/direct-add", gameHandler.AddParticipantDirectly) // GM adds player or audience member directly

				// Co-GM management
				r.Post("/participants/{userId}/promote-to-co-gm", gameHandler.PromoteToCoGM)         // GM promotes audience to co-GM
				r.Post("/participants/{userId}/demote-from-co-gm", gameHandler.DemoteFromCoGM)       // GM demotes co-GM to audience
				r.Post("/participants/{userId}/to-audience", gameHandler.TransitionPlayerToAudience) // GM moves player to audience (permadeath)

				// Game application management
				// Apply to game requires email verification
				r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).Post("/apply", gameHandler.ApplyToGame)
				r.Get("/applications", gameHandler.GetGameApplications)
				r.Get("/application/mine", gameHandler.GetMyGameApplication)
				r.Put("/applications/{applicationId}/review", gameHandler.ReviewGameApplication)
				r.Delete("/application", gameHandler.WithdrawGameApplication)

				// Audience participation
				r.Get("/audience", gameHandler.ListAudienceMembers)
				r.Get("/characters/audience-npcs", gameHandler.ListAudienceNPCs)
				r.Put("/settings/auto-accept-audience", gameHandler.UpdateAutoAcceptAudience)
				r.Get("/private-messages/all", gameHandler.ListAllPrivateConversations)
				r.Get("/private-messages/participants", gameHandler.GetConversationParticipants)
				r.Get("/private-messages/conversations/{conversationId}", gameHandler.GetAudienceConversationMessages)
				r.Get("/action-submissions/all", gameHandler.ListAllActionSubmissions)

				// Post-game statistics (completed games only; handler enforces)
				r.Get("/stats", gameHandler.GetGameStats)

				// Character management within games
				characterHandler := characters.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				// Create character requires email verification
				r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).Post("/characters", characterHandler.CreateCharacter)
				r.Get("/characters", characterHandler.GetGameCharacters)
				r.Get("/characters/stats", characterHandler.GetGameCharacterStats)
				r.Get("/characters/controllable", characterHandler.GetUserControllableCharacters)
				r.Get("/characters/inactive", characterHandler.ListInactiveCharacters) // GM views inactive characters

				// Phase management within games
				phaseHandler := phases.Handler{
					App:                     h.App,
					PhaseService:            &dbphases.PhaseService{DB: h.App.Pool},
					ActionSubmissionService: &dbactions.ActionSubmissionService{DB: h.App.Pool, Logger: h.App.ObsLogger, NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger)},
					GameService:             &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService:     db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				r.Post("/phases", phaseHandler.CreatePhase)
				r.Get("/current-phase", phaseHandler.GetCurrentPhase)
				r.Get("/phases", phaseHandler.GetGamePhases)
				r.Post("/actions", phaseHandler.SubmitAction)
				r.Get("/actions", phaseHandler.GetGameActions)
				r.Get("/actions/mine", phaseHandler.GetUserActions)

				// Action results management
				r.Post("/results", phaseHandler.CreateActionResult)
				r.Get("/results", phaseHandler.GetGameActionResults)
				r.Get("/results/mine", phaseHandler.GetUserActionResults)
				r.Put("/results/{resultId}", phaseHandler.UpdateActionResult)
				r.Delete("/results/{resultId}", phaseHandler.DeleteActionResult)
				r.Post("/results/{resultId}/publish", phaseHandler.PublishActionResult)
				r.Post("/phases/{phaseId}/results/publish", phaseHandler.PublishAllPhaseResults)
				r.Get("/phases/{phaseId}/results/unpublished-count", phaseHandler.GetUnpublishedResultsCount)

				// Draft character updates for action results
				r.Post("/results/{resultId}/character-updates", phaseHandler.CreateDraftCharacterUpdate)
				r.Get("/results/{resultId}/character-updates", phaseHandler.GetDraftCharacterUpdates)
				r.Get("/results/{resultId}/character-updates/count", phaseHandler.GetDraftUpdateCount)
				r.Put("/results/{resultId}/character-updates/{draftId}", phaseHandler.UpdateDraftCharacterUpdate)
				r.Delete("/results/{resultId}/character-updates/{draftId}", phaseHandler.DeleteDraftCharacterUpdate)

				// Common Room messages (posts and comments)
				messageHandler := messages.Handler{
					App:            h.App,
					UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
				}
				// Create post requires email verification
				r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).Post("/posts", messageHandler.CreatePost)
				r.Get("/posts", messageHandler.GetGamePosts)
				r.Patch("/posts/{postId}", messageHandler.UpdatePost) // Edit post
				// Create comment requires email verification
				r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).Post("/posts/{postId}/comments", messageHandler.CreateComment)
				r.Get("/posts/{postId}/comments", messageHandler.GetPostComments)
				r.Get("/posts/{postId}/comments-with-threads", messageHandler.GetPostCommentsWithThreads) // NEW: Paginated with nested replies
				r.Patch("/posts/{postId}/comments/{commentId}", messageHandler.UpdateComment)             // Edit comment
				r.Delete("/posts/{postId}/comments/{commentId}", messageHandler.DeleteComment)            // Delete comment
				r.Get("/messages/{messageId}", messageHandler.GetMessage)                                 // For deep linking to nested comments
				r.Get("/messages/{messageId}/thread-context", messageHandler.GetMessageThreadContext)     // Target + full ancestor chain in one request
				r.Get("/comments/recent", messageHandler.ListRecentCommentsWithParents)                   // New Comments view

				// Read tracking for common room
				r.Post("/posts/{postId}/mark-read", messageHandler.MarkPostRead)
				r.Get("/read-markers", messageHandler.GetGameReadMarkers)
				r.Get("/posts-unread-info", messageHandler.GetPostsUnreadInfo)
				r.Get("/unread-comment-ids", messageHandler.GetUnreadCommentIDs)

				// Manual read tracking (per-comment)
				r.Post("/posts/{postId}/comments/{commentId}/toggle-read", messageHandler.ToggleCommentRead)
				r.Get("/manual-read-comment-ids", messageHandler.GetManualReadCommentIDs)
				r.Post("/phases/{phaseId}/mark-all-comments-read", messageHandler.MarkAllCommentsRead)

				// Private messages (conversations)
				conversationHandler := &conversations.Handler{
					App:                 h.App,
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					ConversationService: db.NewConversationService(h.App.Pool),
					PhaseService:        &dbphases.PhaseService{DB: h.App.Pool},
				}
				conversationHandler.RegisterRoutes(r)

				// Handouts
				handoutHandler := &handouts.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					HandoutService:      db.NewHandoutService(h.App.Pool),
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				r.Post("/handouts", handoutHandler.CreateHandout)
				r.Get("/handouts", handoutHandler.ListHandouts)
				r.Get("/handouts/{handoutId}", handoutHandler.GetHandout)
				r.Put("/handouts/{handoutId}", handoutHandler.UpdateHandout)
				r.Delete("/handouts/{handoutId}", handoutHandler.DeleteHandout)
				r.Post("/handouts/{handoutId}/publish", handoutHandler.PublishHandout)
				r.Post("/handouts/{handoutId}/unpublish", handoutHandler.UnpublishHandout)

				// Game archive exports (completed games only). Read access is
				// CanUserViewGame, so any authenticated user may export a
				// completed game's public archive.
				exportHandler := &exports.Handler{
					App:           h.App,
					UserService:   &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:   &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					ExportService: exports.NewService(h.App.Pool, h.App.Config.Storage.ArchivePath, h.App.ObsLogger),
				}
				r.Post("/exports", exportHandler.RequestExport)
				r.Get("/exports/latest", exportHandler.GetLatestExport)

				// Handout comments
				r.Post("/handouts/{handoutId}/comments", handoutHandler.CreateHandoutComment)
				r.Get("/handouts/{handoutId}/comments", handoutHandler.ListHandoutComments)
				r.Patch("/handouts/{handoutId}/comments/{commentId}", handoutHandler.UpdateHandoutComment)
				r.Delete("/handouts/{handoutId}/comments/{commentId}", handoutHandler.DeleteHandoutComment)

				// Deadlines
				deadlineHandler := &deadlines.Handler{
					App:             h.App,
					UserService:     &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:     &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					DeadlineService: &db.DeadlineService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				}
				r.Post("/deadlines", deadlineHandler.CreateDeadline)
				r.Get("/deadlines", deadlineHandler.GetGameDeadlines)

				// Polls
				pollHandler := &polls.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					PollService:         &db.PollService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				r.Post("/polls", pollHandler.CreatePoll)
				r.Get("/polls", pollHandler.ListGamePolls)
				r.Get("/phases/{phaseId}/polls", pollHandler.ListPollsByPhase)

				// Logs
				r.Get("/logs", gameHandler.GetGameLogs)

				r.Get("/loot-tables", gameHandler.GetGameLootTables)
				r.Post("/loot-tables", gameHandler.AddGameLootTable)
				r.Put("/loot-tables/{tableId}", gameHandler.UpdateGameLootTable)
				r.Delete("/loot-tables/{tableId}", gameHandler.DeleteGameLootTable)
				r.Get("/loot-tables/{tableId}/contents", gameHandler.GetGameLootTableContents)
				r.Post("/loot-tables/{tableId}/contents", gameHandler.UpdateGameLootTableContent)
				r.Post("/loot-tables/{tableId}/random/{characterId}", gameHandler.SetRandomLootForCharacter) // Assign random loot to character from table
			})
		})
	})
	apiV1Router.Mount("/games", gamesRouter)

	// Characters API (for character-specific operations)
	charactersRouter := chi.NewRouter()
	charactersRouter.Route("/", func(r chi.Router) {
		characterHandler := characters.Handler{
			App:                 h.App,
			UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
		}
		avatarHandler := avatars.Handler{
			App:              h.App,
			CharacterService: &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}
		userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}

		// All character routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// Cross-game character list for the current user. Static segment, so
			// chi matches it ahead of the /{id} route below.
			r.Get("/controllable", characterHandler.GetUserControllableCharactersAcrossGames)

			// Character management
			r.Get("/{id}", characterHandler.GetCharacter)
			r.Post("/{id}/approve", characterHandler.ApproveCharacter)
			r.Post("/{id}/assign", characterHandler.AssignNPC)
			r.Put("/{id}/reassign", characterHandler.ReassignCharacter) // GM reassigns inactive character
			r.Put("/{id}/rename", characterHandler.RenameCharacter)     // GM or owner renames character
			r.Delete("/{id}", characterHandler.DeleteCharacter)         // GM deletes character with no activity
			r.Post("/{id}/data", characterHandler.SetCharacterData)
			r.Get("/{id}/data", characterHandler.GetCharacterData)

			// Character activity stats
			r.Get("/{id}/stats", characterHandler.GetCharacterStats)

			// Character page - public activity feed
			messageHandler := messages.Handler{
				App:            h.App,
				UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			}
			r.Get("/{id}/comments", messageHandler.GetCharacterComments)

			// Avatar management
			r.Post("/{id}/avatar", avatarHandler.UploadCharacterAvatar)
			r.Delete("/{id}/avatar", avatarHandler.DeleteCharacterAvatar)
		})
	})
	apiV1Router.Mount("/characters", charactersRouter)

	// Phases API (for phase-specific operations)
	phasesRouter := chi.NewRouter()
	phasesRouter.Route("/", func(r chi.Router) {
		phaseHandler := phases.Handler{
			App:                     h.App,
			PhaseService:            &dbphases.PhaseService{DB: h.App.Pool},
			ActionSubmissionService: &dbactions.ActionSubmissionService{DB: h.App.Pool, Logger: h.App.ObsLogger, NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger)},
			GameService:             &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			NotificationService:     db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
		}

		// All phase routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// Phase management
			r.Post("/{id}/activate", phaseHandler.ActivatePhase)
			r.Put("/{id}/deadline", phaseHandler.UpdatePhaseDeadline)
			r.Put("/{id}", phaseHandler.UpdatePhase)
			r.Delete("/{id}", phaseHandler.DeletePhase)

			// Draft post management (GM only)
			messageHandler := messages.Handler{
				App:            h.App,
				UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			}
			r.Get("/{id}/draft-post", messageHandler.GetDraftPost)
			r.Post("/{id}/draft-post", messageHandler.CreateDraftPost)
			r.Put("/{id}/draft-post", messageHandler.UpdateDraftPost)
			r.Delete("/{id}/draft-post", messageHandler.DeleteDraftPost)
		})
	})
	apiV1Router.Mount("/phases", phasesRouter)

	// Deadlines API (for deadline-specific operations)
	deadlinesRouter := chi.NewRouter()
	deadlinesRouter.Route("/", func(r chi.Router) {
		deadlineHandler := deadlines.Handler{
			App:             h.App,
			UserService:     &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:     &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			DeadlineService: &db.DeadlineService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}

		// All deadline routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// Deadline management
			r.Get("/upcoming", deadlineHandler.GetUpcomingDeadlines) // Get upcoming deadlines across all user's games
			r.Patch("/{deadlineId}", deadlineHandler.UpdateDeadline)
			r.Delete("/{deadlineId}", deadlineHandler.DeleteDeadline)
		})
	})
	apiV1Router.Mount("/deadlines", deadlinesRouter)

	// Game archive export downloads. Not nested under /games because the export
	// id is the addressable resource; the handler resolves the game from the
	// export row and re-checks CanUserViewGame, so a leaked export id grants
	// nothing the caller could not already read.
	exportsRouter := chi.NewRouter()
	exportsRouter.Route("/", func(r chi.Router) {
		exportHandler := &exports.Handler{
			App:           h.App,
			UserService:   &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:   &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			ExportService: exports.NewService(h.App.Pool, h.App.Config.Storage.ArchivePath, h.App.ObsLogger),
		}

		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			r.Get("/{exportID}/download", exportHandler.DownloadExport)
		})
	})
	apiV1Router.Mount("/exports", exportsRouter)

	// Polls API (for poll-specific operations)
	pollsRouter := chi.NewRouter()
	pollsRouter.Route("/", func(r chi.Router) {
		pollHandler := polls.Handler{
			App:                 h.App,
			UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			PollService:         &db.PollService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
		}

		// All poll routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// Poll management
			r.Get("/{pollId}", pollHandler.GetPoll)
			r.Get("/{pollId}/results", pollHandler.GetPollResults)
			r.Post("/{pollId}/vote", pollHandler.SubmitVote)
			r.Put("/{pollId}", pollHandler.UpdatePoll)
			r.Delete("/{pollId}", pollHandler.DeletePoll)
		})
	})
	apiV1Router.Mount("/polls", pollsRouter)

	// Notifications API
	notificationsRouter := chi.NewRouter()
	notificationsRouter.Route("/", func(r chi.Router) {
		notificationHandler := notifications.Handler{
			App:                 h.App,
			NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
		}

		// All notification routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))

			// Notification management
			r.Get("/", notificationHandler.GetNotifications)
			r.Get("/unread-count", notificationHandler.GetUnreadCount)
			r.Put("/mark-all-read", notificationHandler.MarkAllAsRead)
			r.Get("/{id}", notificationHandler.GetNotification)
			r.Put("/{id}/mark-read", notificationHandler.MarkNotificationAsRead)
			r.Put("/{id}/mark-unread", notificationHandler.MarkNotificationAsUnread)
			r.Delete("/{id}", notificationHandler.DeleteNotification)
		})
	})
	apiV1Router.Mount("/notifications", notificationsRouter)

	// Dashboard API
	dashboardRouter := chi.NewRouter()
	dashboardRouter.Route("/", func(r chi.Router) {
		dashboardHandler := dashboard.Handler{
			App:              h.App,
			UserService:      &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			DashboardService: &db.DashboardService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}

		// Dashboard route requires authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))

			// Get user's dashboard
			r.Get("/", dashboardHandler.GetUserDashboard)
		})
	})
	apiV1Router.Mount("/dashboard", dashboardRouter)

	// Users API - User profiles and avatars
	usersRouter := chi.NewRouter()
	usersRouter.Route("/", func(r chi.Router) {
		userHandler := users.Handler{
			App:         h.App,
			UserService: &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
		}

		// All user profile routes require authentication
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))

			// Profile viewing (public - any authenticated user can view any profile)
			r.Get("/{id}/profile", userHandler.GetUserProfile)
			r.Get("/username/{username}/profile", userHandler.GetUserProfileByUsername)

			// Profile editing (own profile only)
			r.Patch("/me/profile", userHandler.UpdateUserProfile)

			// Avatar management (own profile only)
			r.Post("/me/avatar", userHandler.UploadUserAvatar)
			r.Delete("/me/avatar", userHandler.DeleteUserAvatar)
		})
	})
	apiV1Router.Mount("/users", usersRouter)

	// Admin API - All routes require authentication AND admin privileges
	adminRouter := chi.NewRouter()
	adminRouter.Route("/", func(r chi.Router) {
		adminHandler := admin.Handler{
			App:                   h.App,
			UserService:           &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			SessionService:        &db.SessionService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			IPBanService:          &db.IPBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			FingerprintBanService: &db.FingerprintBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			MessageService:        &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
		}

		// All admin routes require authentication and admin privileges
		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(httpmiddleware.RequireAdmin(h.App))

			// Admin management
			r.Get("/admins", adminHandler.ListAdmins)

			// User admin status management
			r.Put("/users/{id}/admin", adminHandler.GrantAdminStatus)
			r.Delete("/users/{id}/admin", adminHandler.RevokeAdminStatus)

			// User banning
			r.Post("/users/{id}/ban", adminHandler.BanUser)
			r.Delete("/users/{id}/ban", adminHandler.UnbanUser)
			r.Get("/users/banned", adminHandler.ListBannedUsers)

			// User list, pending approval, sessions (fixed paths before parameterized)
			r.Get("/users", adminHandler.ListUsers)
			r.Get("/users/pending", adminHandler.ListPendingApprovalUsers)
			r.Post("/users/{id}/approve", adminHandler.ApproveUser)
			r.Post("/users/{id}/reject", adminHandler.RejectUser)
			r.Get("/users/{id}/sessions", adminHandler.GetUserSessions)

			// IP bans
			r.Get("/ip-bans", adminHandler.ListIPBans)
			r.Post("/ip-bans", adminHandler.CreateIPBan)
			r.Delete("/ip-bans/{id}", adminHandler.DeleteIPBan)

			// Device fingerprint bans
			r.Get("/fingerprint-bans", adminHandler.ListFingerprintBans)
			r.Post("/fingerprint-bans", adminHandler.CreateFingerprintBan)
			r.Delete("/fingerprint-bans/{id}", adminHandler.DeleteFingerprintBan)

			// Content moderation
			r.Delete("/messages/{messageId}", adminHandler.DeleteMessage)
		})
	})
	apiV1Router.Mount("/admin", adminRouter)

	// API Documentation routes (public) - register on apiV1Router BEFORE mounting
	docsHandler := &docs.Handler{}
	docsHandler.RegisterRoutes(apiV1Router)

	// Debug routes (development only) - exposed via /api/v1/debug/*
	if h.App.Config.App.Environment == "development" {
		debugHandler := &DebugHandler{}
		apiV1Router.Route("/debug", func(r chi.Router) {
			debugHandler.RegisterRoutes(r)
		})
	}

	r.Mount("/api/v1", apiV1Router)

	// Serve static documentation at /docs
	docs.RegisterStaticDocs(r, h.App.ObsLogger)

	// Serve static files for local storage (only when using LocalStorage backend)
	// S3 storage serves files directly from S3, so we only need this for local development
	if h.App.Config.Storage.Backend == "local" {
		workDir, _ := os.Getwd()
		filesDir := http.Dir(filepath.Join(workDir, h.App.Config.Storage.LocalPath))
		h.App.Logger.Info("Serving static files",
			"path", "/uploads",
			"directory", filesDir)

		// Use FileServer to serve files from the uploads directory
		fileServer := http.FileServer(filesDir)
		r.Get("/uploads/*", func(w http.ResponseWriter, r *http.Request) {
			// Strip /uploads prefix and serve the file
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/uploads")
			fileServer.ServeHTTP(w, r)
		})
	}

	// Wrap the router with OpenTelemetry HTTP instrumentation.
	// This creates spans for every request when OTEL_ENABLED=true.
	// When OTEL is disabled, the global provider is a no-op so this is zero cost.
	// Span names are set to the chi route template (e.g. "GET /api/v1/games/{id}")
	// by RouteTagMiddleware, which runs after chi has matched the route.
	otelHandler := otelhttp.NewHandler(r, "actionphase-http")

	// Create HTTP server with configuration
	server := &http.Server{
		Addr:         h.App.Config.GetServerAddress(),
		Handler:      otelHandler,
		ReadTimeout:  h.App.Config.Server.ReadTimeout,
		WriteTimeout: h.App.Config.Server.WriteTimeout,
		IdleTimeout:  h.App.Config.Server.IdleTimeout,
	}

	h.App.Logger.Info("HTTP server starting",
		"address", server.Addr,
		"read_timeout", server.ReadTimeout,
		"write_timeout", server.WriteTimeout)

	// Background job: delete notifications older than 30 days, runs once per day
	go func() {
		notificationService := db.NewNotificationService(h.App.Pool, h.App.ObsLogger)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := notificationService.DeleteOldReadNotifications(context.Background()); err != nil {
				h.App.ObsLogger.Error(context.Background(), "Background notification cleanup failed", "error", err)
			}
		}
	}()

	// Background job: auth table cleanup (tokens, verification records, bot data)
	go func() {
		passwordSvc := &auth.PasswordService{DB: h.App.Pool, Logger: h.App.ObsLogger}
		accountSvc := &auth.AccountService{DB: h.App.Pool, Logger: h.App.ObsLogger}
		botPreventionSvc := auth.NewBotPreventionService(h.App.Pool)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			if err := passwordSvc.CleanupExpiredTokens(ctx); err != nil {
				h.App.ObsLogger.Error(ctx, "Background password token cleanup failed", "error", err)
			}
			if err := accountSvc.CleanupExpiredVerificationTokens(ctx); err != nil {
				h.App.ObsLogger.Error(ctx, "Background verification token cleanup failed", "error", err)
			}
			if err := botPreventionSvc.CleanupOldRegistrationAttempts(ctx); err != nil {
				h.App.ObsLogger.Error(ctx, "Background registration attempt cleanup failed", "error", err)
			}
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		h.App.Logger.Error("HTTP server failed", "error", err)
	}
}
