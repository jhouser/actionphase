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
	dbcommunities "actionphase/pkg/db/services/communities"
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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
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

// Start builds the router and serves it.
func (h *Handler) Start() {
	r, _ := h.Router()

	// Wrap the router with OpenTelemetry HTTP instrumentation.
	// This creates spans for every request when OTEL_ENABLED=true.
	// When OTEL is disabled, the global provider is a no-op so this is zero cost.
	// Span names are set to the chi route template (e.g. "GET /api/v1/games/{id}")
	// by RouteTagMiddleware, which runs after chi has matched the route.
	otelHandler := otelhttp.NewHandler(r, "actionphase-http")

	h.serve(otelHandler)
}

// Router builds the complete route tree and returns it alongside the docs
// handler that describes it.
//
// Split out of Start so the OpenAPI document can be rendered without listening
// on a port: `just gen-openapi` calls this, takes the docs handler, and writes
// the same bytes the server would serve. That is what lets check-api-docs be a
// diff against a committed file rather than a heuristic comparison of the
// router source against a hand-written spec.
func (h *Handler) Router() (chi.Router, *docs.Handler) {
	// Huma's default error body is RFC 7807, which the frontend cannot parse.
	// Install the legacy shape before any huma API is built. See
	// .claude/planning/rfc7807-error-format.md.
	InstallLegacyErrorFormat()

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

	// One huma API per middleware group under /auth; see the groups below.
	var (
		authPublicAPI               huma.API
		authRateLimitedAPI          huma.API
		authProbeAPI                huma.API
		authProtectedAPI            huma.API
		authRateLimitedProtectedAPI huma.API
	)
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

		// In development mode, rate limiting is relaxed for E2E testing.
		isDev := h.App.Config.IsDevelopment()

		// Auth's routes do not share one middleware stack, and huma binds an
		// API to a chi router -- so each group gets its own API and inherits
		// that group's middleware unchanged. generatedSpecFor merges the five
		// documents under the single /auth prefix.

		// Public, unlimited.
		r.Group(func(r chi.Router) {
			authPublicAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			auth.RegisterHumaAuthPublic(authPublicAPI, &authHandler)
		})

		// Public, strictly rate limited: the credential-guessing surface.
		r.Group(func(r chi.Router) {
			r.Use(ratelimitmw.StrictRateLimit(isDev))
			authRateLimitedAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			auth.RegisterHumaAuthRateLimited(authRateLimitedAPI, &authHandler)
		})

		// Probe: Verifier only, deliberately no Authenticator, so /me answers
		// 200 with a null user rather than 401 when the caller is anonymous.
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(h.getTokenAuth()))
			authProbeAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			auth.RegisterHumaAuthProbe(authProbeAPI, &authHandler)
		})

		// Discord OAuth callback (public — Discord redirects here after
		// authorization). Left on chi: it answers a 302 redirect and writes
		// plain-text errors, so there is no JSON shape to generate, the same
		// reasoning that leaves /ping unconverted.
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

			authProtectedAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			auth.RegisterHumaAuthProtected(authProtectedAPI, &authHandler)

			// Authenticated *and* rate limited, so it needs its own group.
			r.Group(func(r chi.Router) {
				r.Use(ratelimitmw.StrictRateLimit(isDev))
				authRateLimitedProtectedAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
				auth.RegisterHumaAuthRateLimitedProtected(authRateLimitedProtectedAPI, &authHandler)
			})
		})
	})
	apiV1Router.Mount("/auth", authRouter)

	// Games API - All routes require authentication
	gamesRouter := chi.NewRouter()
	// gameScopedAPI is the single huma API for the /{gameID} subrouter. Several
	// packages hang operations off that mount, and they must share one API:
	// generatedSpecFor keys by mount prefix, so a second API on the same prefix
	// would be dropped from the served spec.
	var gameScopedAPI huma.API
	var gamesAPI, gamesPublicListAPI, gamesPublicGameAPI huma.API
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
			// Verifier without Authenticator: a token is read if present, and
			// its absence is not an error. The listing is simply unenriched for
			// an anonymous caller.
			r.Use(jwtauth.Verifier(tokenAuth))

			// Two APIs, not one, because the applicants route additionally needs
			// GameMiddleware to load the game and the listing must not have it.
			// A huma API binds to one router, so a differing middleware stack
			// needs its own (gotcha 19).
			gamesPublicListAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			games.RegisterHumaGamesPublicList(gamesPublicListAPI, &gameHandler)

			r.Group(func(r chi.Router) {
				r.Use(gameHandler.GameMiddleware())
				gamesPublicGameAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
				games.RegisterHumaGamesPublicApplicants(gamesPublicGameAPI, &gameHandler)
			})
		})

		// All routes below require authentication
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// huma / type-first -- the collection routes (/recruiting and
			// POST /), which need no game context. The game-scoped operations
			// register on gameScopedAPI inside the /{gameID} subrouter below,
			// where GameMiddleware has loaded the game.
			//
			// Email verification for create-game now runs inside the handler
			// (core.RequireVerifiedEmailCtx), since huma handlers take a
			// context rather than a *http.Request (gotcha 15).
			gamesAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			games.RegisterHumaGamesCollection(gamesAPI, &gameHandler)

			r.Route("/{gameID}", func(r chi.Router) {
				r.Use(gameHandler.GameMiddleware())

				// Shared by every converted package registering on this
				// subrouter; see the declaration for why it must be one API.
				gameScopedAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")

				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				// Covers the game, participant, application, audience, log,
				// stats, banner and loot-table operations. The email
				// verification apply-to-game required now runs inside that
				// handler (core.RequireVerifiedEmailCtx), since huma handlers
				// take a context rather than a *http.Request.
				games.RegisterHumaGameScoped(gameScopedAPI, &gameHandler)

				// Character management within games
				characterHandler := characters.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter. The
				// email-verification requirement on create now runs inside the
				// handler (core.RequireVerifiedEmailCtx), since huma handlers
				// take a context rather than a *http.Request.
				characters.RegisterHumaGameCharacters(gameScopedAPI, &characterHandler)

				// Phase management within games
				phaseHandler := phases.Handler{
					App:                     h.App,
					PhaseService:            &dbphases.PhaseService{DB: h.App.Pool},
					ActionSubmissionService: &dbactions.ActionSubmissionService{DB: h.App.Pool, Logger: h.App.ObsLogger, NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger)},
					GameService:             &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService:     db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				// Registers the action, result, staged-chain and draft
				// character-update operations too.
				phases.RegisterHumaGamePhases(gameScopedAPI, &phaseHandler)

				// Common Room messages (posts and comments)
				messageHandler := messages.Handler{
					App:            h.App,
					UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
				}
				// Create post requires email verification
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter. The
				// email-verification requirement on create-post and
				// create-comment now runs inside those handlers
				// (core.RequireVerifiedEmailCtx), since huma handlers take a
				// context rather than a *http.Request.
				messages.RegisterHumaGameMessages(gameScopedAPI, &messageHandler)

				// Private messages (conversations)
				conversationHandler := &conversations.Handler{
					App:                 h.App,
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					ConversationService: db.NewConversationService(h.App.Pool, h.App.ObsLogger),
					PhaseService:        &dbphases.PhaseService{DB: h.App.Pool},
				}
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				conversations.RegisterHumaConversations(gameScopedAPI, conversationHandler)

				// Handouts
				handoutHandler := &handouts.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					HandoutService:      db.NewHandoutService(h.App.Pool),
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				// Registers the comment operations too.
				handouts.RegisterHumaGameHandouts(gameScopedAPI, handoutHandler)

				// Game archive exports (completed games only). Read access is
				// CanUserViewGame, so any authenticated user may export a
				// completed game's public archive.
				exportHandler := &exports.Handler{
					App:           h.App,
					UserService:   &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:   &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					ExportService: exports.NewService(h.App.Pool, h.App.Config.Storage.ArchivePath, h.App.ObsLogger),
				}
				// huma / type-first — paths are relative to this /{gameID}
				// subrouter (see huma-migration.md gotcha 4).
				exports.RegisterHumaGameExports(gameScopedAPI, exportHandler)

				// Deadlines
				deadlineHandler := &deadlines.Handler{
					App:             h.App,
					UserService:     &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:     &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					DeadlineService: &db.DeadlineService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				}
				// huma / type-first — shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				deadlines.RegisterHumaGameDeadlines(gameScopedAPI, deadlineHandler)

				// Polls
				pollHandler := &polls.Handler{
					App:                 h.App,
					UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					PollService:         &db.PollService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					CharacterService:    &db.CharacterService{DB: h.App.Pool, Logger: h.App.ObsLogger},
					NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
				}
				// huma / type-first -- shares gameScopedAPI with the other
				// packages registering on this same /{gameID} subrouter.
				polls.RegisterHumaGamePolls(gameScopedAPI, pollHandler)
			})
		})
	})
	apiV1Router.Mount("/games", gamesRouter)

	// Characters API (for character-specific operations).
	// charactersAPI is the single huma API for this mount: the characters and
	// avatars packages both register onto it.
	var charactersAPI huma.API
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

			// One huma API for this whole /characters mount: characters and
			// avatars both register onto it, because generatedSpecFor keys by
			// mount prefix and a second API here would be dropped.
			charactersAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")

			// huma / type-first -- registers the cross-game /controllable list
			// and every per-character operation.
			characters.RegisterHumaCharacters(charactersAPI, &characterHandler)

			// Character page - public activity feed
			messageHandler := messages.Handler{
				App:            h.App,
				UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			}
			// huma / type-first -- shares charactersAPI with the characters
			// and avatars packages on this same /characters mount.
			messages.RegisterHumaCharacterMessages(charactersAPI, &messageHandler)

			// Avatar management (huma / type-first)
			avatars.RegisterHumaAvatars(charactersAPI, &avatarHandler)
		})
	})
	apiV1Router.Mount("/characters", charactersRouter)

	// Phases API (for phase-specific operations)
	var phasesAPI huma.API
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

			// Draft post management (GM only)
			messageHandler := messages.Handler{
				App:            h.App,
				UserService:    &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
				MessageService: &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			}
			// huma / type-first -- paths are relative to this /phases mount.
			// Both pkg/phases and pkg/messages register on this one API, since
			// they share the router and its middleware (gotcha 3).
			phasesAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			phases.RegisterHumaPhases(phasesAPI, &phaseHandler)
			messages.RegisterHumaPhaseDraftPosts(phasesAPI, &messageHandler)
		})
	})
	apiV1Router.Mount("/phases", phasesRouter)

	// Deadlines API (for deadline-specific operations)
	var deadlinesAPI huma.API
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

			// Deadline management (huma / type-first)
			deadlinesAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			deadlines.RegisterHumaDeadlines(deadlinesAPI, &deadlineHandler)
		})
	})
	apiV1Router.Mount("/deadlines", deadlinesRouter)

	// Cross-game handout list for the current user. Not nested under /games
	// because it spans every game the user is in, which is exactly what the
	// global Utility Drawer needs when no game is in scope. Per-game handout
	// routes remain under /games/{gameID}/handouts.
	var handoutsAPI huma.API
	handoutsRouter := chi.NewRouter()
	handoutsRouter.Route("/", func(r chi.Router) {
		handoutHandler := &handouts.Handler{
			App:                 h.App,
			UserService:         &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			GameService:         &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			HandoutService:      db.NewHandoutService(h.App.Pool),
			NotificationService: db.NewNotificationService(h.App.Pool, h.App.ObsLogger),
		}

		r.Group(func(r chi.Router) {
			tokenAuth := h.getTokenAuth()
			userService := &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger}
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(h.sessionValidateMW())
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			// huma / type-first -- paths are relative to this /handouts mount.
			handoutsAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			handouts.RegisterHumaHandouts(handoutsAPI, handoutHandler)
		})
	})
	apiV1Router.Mount("/handouts", handoutsRouter)

	// Game archive export downloads. Not nested under /games because the export
	// id is the addressable resource; the handler resolves the game from the
	// export row and re-checks CanUserViewGame, so a leaked export id grants
	// nothing the caller could not already read.
	var exportDownloadsAPI huma.API
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

			exportDownloadsAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			exports.RegisterHumaExportDownloads(exportDownloadsAPI, exportHandler)
		})
	})
	apiV1Router.Mount("/exports", exportsRouter)

	// Polls API (for poll-specific operations)
	var pollsAPI huma.API
	pollsRouter := chi.NewRouter()
	pollsRouter.Route("/", func(r chi.Router) {
		pollHandler := &polls.Handler{
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
			pollsAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			polls.RegisterHumaPolls(pollsAPI, pollHandler)
		})
	})
	apiV1Router.Mount("/polls", pollsRouter)

	// Notifications API (huma / type-first -- see .claude/planning/huma-migration.md)
	var notificationsAPI huma.API
	notificationsRouter := chi.NewRouter()
	notificationsRouter.Route("/", func(r chi.Router) {
		notificationHandler := &notifications.Handler{
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

			notificationsAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			notifications.RegisterHumaNotifications(notificationsAPI, notificationHandler)
		})
	})
	apiV1Router.Mount("/notifications", notificationsRouter)

	// Dashboard API (huma / type-first — see .claude/planning/huma-migration.md)
	var dashboardAPI huma.API
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

			dashboardAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			dashboard.RegisterHumaDashboard(dashboardAPI, &dashboardHandler)
		})
	})
	apiV1Router.Mount("/dashboard", dashboardRouter)

	// Users API - User profiles and avatars
	var usersAPI huma.API
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

			// Profile viewing, profile editing and avatar management
			// (huma / type-first)
			usersAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			users.RegisterHumaUsers(usersAPI, &userHandler)
		})
	})
	apiV1Router.Mount("/users", usersRouter)

	// Admin API - All routes require authentication AND admin privileges.
	// adminAPI is captured so the docs handler can serve huma's generated spec
	// for these routes (see below).
	var adminAPI huma.API
	adminRouter := chi.NewRouter()
	adminRouter.Route("/", func(r chi.Router) {
		adminHandler := admin.Handler{
			App:                   h.App,
			UserService:           &db.UserService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			SessionService:        &db.SessionService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			IPBanService:          &db.IPBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			FingerprintBanService: &db.FingerprintBanService{DB: h.App.Pool, Logger: h.App.ObsLogger},
			MessageService:        &dbmessages.MessageService{DB: h.App.Pool, Logger: h.App.ObsLogger, Metrics: h.App.Observability.OTELMetrics},
			CommunityService:      &dbcommunities.CommunityService{DB: h.App.Pool, Logger: h.App.ObsLogger},
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

			// Admin routes are type-first (huma): paths, params, schemas and
			// status codes are derived from the Go types in
			// pkg/admin/huma_api.go, so the OpenAPI spec cannot drift from
			// the handlers. Huma mounts onto this same chi router, so the
			// middleware above applies unchanged.
			//
			// Registered on adminRouter (not r) because huma paths are
			// absolute and this group is mounted at /api/v1/admin.
			adminAPI = newHumaAPI(r, "ActionPhase API", "1.0.0")
			admin.RegisterHumaAdmin(adminAPI, &adminHandler)
		})
	})
	apiV1Router.Mount("/admin", adminRouter)

	// API Documentation routes (public) - register on apiV1Router BEFORE mounting
	// The served spec merges huma's generated paths over the hand-written
	// openapi.yaml, so migrated packages are documented from their Go types
	// and the rest fall back to the manual file. Each package converted in
	// .claude/planning/huma-migration.md improves the docs automatically.
	docsHandler := &docs.Handler{
		GeneratedSpec: func() ([]byte, error) {
			return generatedSpecFor(map[string][]huma.API{
				"/admin":      {adminAPI},
				"/dashboard":  {dashboardAPI},
				"/characters": {charactersAPI},
				"/exports":    {exportDownloadsAPI},
				// Registered on the /{gameID} subrouter, so the documented URL
				// needs that segment added back.
				"/deadlines":      {deadlinesAPI},
				"/users":          {usersAPI},
				"/notifications":  {notificationsAPI},
				"/polls":          {pollsAPI},
				"/handouts":       {handoutsAPI},
				"/phases":         {phasesAPI},
				"/games/{gameID}": {gameScopedAPI},
				// Three APIs, one mount: the /games routes split across chi
				// groups with different middleware (the public listing runs
				// Verifier only, the public applicants list adds GameMiddleware,
				// and the collection routes are fully authenticated). Their
				// paths are disjoint.
				"/games": {gamesAPI, gamesPublicListAPI, gamesPublicGameAPI},
				// Four APIs, one mount: /auth's routes split across chi groups
				// with different middleware (rate-limited, public, Verifier-only
				// probe, fully protected). Their paths are disjoint.
				"/auth": {authPublicAPI, authRateLimitedAPI, authProbeAPI, authProtectedAPI, authRateLimitedProtectedAPI},
			})
		},
	}
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

	return r, docsHandler
}

// serve runs the HTTP server until it stops.
func (h *Handler) serve(handler http.Handler) {
	// Create HTTP server with configuration
	server := &http.Server{
		Addr:         h.App.Config.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  h.App.Config.Server.ReadTimeout,
		WriteTimeout: h.App.Config.Server.WriteTimeout,
		IdleTimeout:  h.App.Config.Server.IdleTimeout,
	}

	h.App.Logger.Info("HTTP server starting",
		"address", server.Addr,
		"read_timeout", server.ReadTimeout,
		"write_timeout", server.WriteTimeout)

	// Background housekeeping workers are started in main.go, where the
	// cancellable process context lives alongside the scheduler and export
	// workers. Starting them here would strand them on context.Background(),
	// making their ctx.Done() case unreachable.

	if err := server.ListenAndServe(); err != nil {
		h.App.Logger.Error("HTTP server failed", "error", err)
	}
}
