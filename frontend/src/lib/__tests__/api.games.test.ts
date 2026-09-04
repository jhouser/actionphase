import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import type {
  Game,
  GameWithDetails,
  GameListItem,
  GameParticipant,
  CreateGameRequest,
  UpdateGameRequest,
  UpdateGameStateRequest,
  ApplyToGameRequest,
  GameApplication,
  ReviewApplicationRequest
} from '../../types/games'

// Mock axios
vi.mock('axios')
const mockAxios = vi.mocked(axios)

// Setup default axios mock before importing apiClient
const mockClient = {
  post: vi.fn(),
  get: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
}

const mockRefreshClient = {
  get: vi.fn(),
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
}

mockAxios.create.mockReturnValueOnce(mockClient).mockReturnValueOnce(mockRefreshClient)

import { apiClient } from '../api'

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
}
Object.defineProperty(window, 'localStorage', {
  value: localStorageMock
})

// Mock console to avoid test output noise
const mockConsole = {
  error: vi.fn(),
  log: vi.fn(),
}
Object.defineProperty(console, 'error', { value: mockConsole.error })
Object.defineProperty(console, 'log', { value: mockConsole.log })

describe('ApiClient - Games', () => {
  let mockPost: ReturnType<typeof vi.fn>
  let mockGet: ReturnType<typeof vi.fn>
  let mockPut: ReturnType<typeof vi.fn>
  let mockDelete: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()

    // Setup axios mock responses
    mockPost = vi.fn()
    mockGet = vi.fn()
    mockPut = vi.fn()
    mockDelete = vi.fn()

    // Update the existing mocked client methods
    mockClient.post = mockPost
    mockClient.get = mockGet
    mockClient.put = mockPut
    mockClient.delete = mockDelete
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('Game listing endpoints', () => {
    describe('getAllGames', () => {
      it('should fetch all public games', async () => {
        const mockGames: GameListItem[] = [
          {
            id: 1,
            title: 'Test Game 1',
            description: 'A test game',
            gm_user_id: 1,
            state: 'recruiting',
            genre: 'Fantasy',
            max_players: 5,
            current_players: 2,
            is_public: true,
            start_date: '2025-09-01T19:00:00Z',
            recruitment_deadline: '2025-08-25T23:59:59Z',
            created_at: '2025-08-07T18:30:00Z',
            updated_at: '2025-08-07T18:30:00Z',
          },
        ]

        const expectedResponse: AxiosResponse<GameListItem[]> = {
          data: mockGames,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getAllGames()

        expect(mockGet).toHaveBeenCalledWith('/api/v1/games/public')
        expect(result).toEqual(expectedResponse)
        expect(result.data).toHaveLength(1)
        expect(result.data[0].title).toBe('Test Game 1')
      })

      it('should handle empty games list', async () => {
        const expectedResponse: AxiosResponse<GameListItem[]> = {
          data: [],
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getAllGames()

        expect(result.data).toHaveLength(0)
      })

      it('should handle API errors', async () => {
        const apiError = {
          response: {
            status: 500,
            data: { detail: 'Internal server error' },
          },
        }

        mockGet.mockRejectedValue(apiError)

        await expect(apiClient.getAllGames()).rejects.toEqual(apiError)
      })
    })

    describe('getRecruitingGames', () => {
      it('should fetch only recruiting games', async () => {
        const mockRecruitingGames: GameListItem[] = [
          {
            id: 2,
            title: 'Recruiting Game',
            description: 'A game accepting players',
            gm_user_id: 1,
            state: 'recruiting',
            genre: 'Sci-Fi',
            max_players: 6,
            current_players: 3,
            is_public: true,
            start_date: '2025-09-15T19:00:00Z',
            recruitment_deadline: '2025-09-01T23:59:59Z',
            created_at: '2025-08-07T18:30:00Z',
            updated_at: '2025-08-07T18:30:00Z',
          },
        ]

        const expectedResponse: AxiosResponse<GameListItem[]> = {
          data: mockRecruitingGames,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getRecruitingGames()

        expect(mockGet).toHaveBeenCalledWith('/api/v1/games/recruiting')
        expect(result).toEqual(expectedResponse)
        expect(result.data[0].state).toBe('recruiting')
      })
    })
  })

  describe('Individual game endpoints', () => {
    describe('getGame', () => {
      it('should fetch game by ID', async () => {
        const gameId = 123
        const mockGame: Game = {
          id: gameId,
          title: 'Specific Game',
          description: 'A specific game',
          gm_user_id: 1,
          state: 'active',
          genre: 'Horror',
          max_players: 4,
          start_date: '2025-09-01T19:00:00Z',
          recruitment_deadline: '2025-08-25T23:59:59Z',
          is_public: true,
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T18:30:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockGame,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getGame(gameId)

        expect(mockGet).toHaveBeenCalledWith(`/api/v1/games/${gameId}`)
        expect(result).toEqual(expectedResponse)
        expect(result.data.id).toBe(gameId)
      })

      it('should handle game not found', async () => {
        const gameId = 999
        const notFoundError = {
          response: {
            status: 404,
            data: { detail: 'Game not found' },
          },
        }

        mockGet.mockRejectedValue(notFoundError)

        await expect(apiClient.getGame(gameId)).rejects.toEqual(notFoundError)
        expect(mockGet).toHaveBeenCalledWith(`/api/v1/games/${gameId}`)
      })
    })

    describe('getGameWithDetails', () => {
      it('should fetch game with additional details', async () => {
        const gameId = 456
        const mockGameDetails: GameWithDetails = {
          id: gameId,
          title: 'Detailed Game',
          description: 'A game with details',
          gm_user_id: 1,
          state: 'active',
          genre: 'Mystery',
          max_players: 5,
          start_date: '2025-09-01T19:00:00Z',
          recruitment_deadline: '2025-08-25T23:59:59Z',
          is_public: true,
          current_phase_id: 1,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T18:30:00Z',
          participants: [],
          current_phase: null,
          characters: [],
        }

        const expectedResponse: AxiosResponse<GameWithDetails> = {
          data: mockGameDetails,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getGameWithDetails(gameId)

        expect(mockGet).toHaveBeenCalledWith(`/api/v1/games/${gameId}/details`)
        expect(result).toEqual(expectedResponse)
        expect(result.data.participants).toBeDefined()
      })
    })

    describe('getGameParticipants', () => {
      it('should fetch game participants', async () => {
        const gameId = 789
        const mockParticipants: GameParticipant[] = [
          {
            user_id: 1,
            username: 'player1',
            role: 'player',
            joined_at: '2025-08-07T18:30:00Z',
          },
          {
            user_id: 2,
            username: 'player2',
            role: 'player',
            joined_at: '2025-08-08T10:15:00Z',
          },
        ]

        const expectedResponse: AxiosResponse<GameParticipant[]> = {
          data: mockParticipants,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getGameParticipants(gameId)

        expect(mockGet).toHaveBeenCalledWith(`/api/v1/games/${gameId}/participants`)
        expect(result).toEqual(expectedResponse)
        expect(result.data).toHaveLength(2)
      })
    })
  })

  describe('Game management endpoints', () => {
    describe('createGame', () => {
      it('should create a new game', async () => {
        const createData: CreateGameRequest = {
          title: 'New Adventure',
          description: 'An exciting new RPG campaign',
          genre: 'Fantasy',
          max_players: 6,
          start_date: '2025-09-01T19:00:00Z',
          recruitment_deadline: '2025-08-25T23:59:59Z',
        }

        const mockCreatedGame: Game = {
          id: 999,
          ...createData,
          gm_user_id: 1,
          state: 'setup',
          is_public: true,
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T18:30:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockCreatedGame,
          status: 201,
          statusText: 'Created',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPost.mockResolvedValue(expectedResponse)

        const result = await apiClient.createGame(createData)

        expect(mockPost).toHaveBeenCalledWith('/api/v1/games', createData)
        expect(result).toEqual(expectedResponse)
        expect(result.data.id).toBe(999)
        expect(result.data.title).toBe('New Adventure')
      })

      it('should handle validation errors', async () => {
        const invalidData: CreateGameRequest = {
          title: '',
          description: '',
          genre: '',
          max_players: 0,
          start_date: '',
          recruitment_deadline: '',
        }

        const validationError = {
          response: {
            status: 422,
            data: {
              detail: 'Validation failed',
              details: {
                title: 'Title is required',
                max_players: 'Must be at least 1',
              }
            },
          },
        }

        mockPost.mockRejectedValue(validationError)

        await expect(apiClient.createGame(invalidData)).rejects.toEqual(validationError)
      })

      it('should send dates in ISO 8601 format (regression test)', async () => {
        // This test prevents regression of the bug where dates were sent in
        // "November 10, 2025 12:00 AM" format instead of ISO 8601 format
        const createData: CreateGameRequest = {
          title: 'Date Format Test Game',
          description: 'Testing proper date formatting',
          genre: 'Testing',
          max_players: 4,
          start_date: '2025-11-10T00:00:00Z', // ISO 8601 format
          end_date: '2025-11-20T00:00:00Z',   // ISO 8601 format
          recruitment_deadline: '2025-11-05T23:59:00Z', // ISO 8601 format
        }

        const mockCreatedGame: Game = {
          id: 1001,
          ...createData,
          gm_user_id: 1,
          state: 'setup',
          is_public: true,
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T18:30:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockCreatedGame,
          status: 201,
          statusText: 'Created',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPost.mockResolvedValue(expectedResponse)

        await apiClient.createGame(createData)

        // Verify the API was called with properly formatted ISO 8601 dates
        expect(mockPost).toHaveBeenCalledWith('/api/v1/games', createData)
        const callData = mockPost.mock.calls[0][1] as CreateGameRequest

        // Verify dates are in ISO 8601 format (YYYY-MM-DDTHH:mm:ssZ)
        expect(callData.start_date).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
        expect(callData.end_date).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
        expect(callData.recruitment_deadline).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
      })
    })

    describe('updateGame', () => {
      it('should update game information', async () => {
        const gameId = 555
        const updateData: UpdateGameRequest = {
          title: 'Updated Game Title',
          description: 'Updated description',
          max_players: 8,
        }

        const mockUpdatedGame: Game = {
          id: gameId,
          title: 'Updated Game Title',
          description: 'Updated description',
          gm_user_id: 1,
          state: 'setup',
          genre: 'Fantasy',
          max_players: 8,
          start_date: '2025-09-01T19:00:00Z',
          recruitment_deadline: '2025-08-25T23:59:59Z',
          is_public: true,
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T19:45:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockUpdatedGame,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPut.mockResolvedValue(expectedResponse)

        const result = await apiClient.updateGame(gameId, updateData)

        expect(mockPut).toHaveBeenCalledWith(`/api/v1/games/${gameId}`, updateData)
        expect(result).toEqual(expectedResponse)
        expect(result.data.title).toBe('Updated Game Title')
        expect(result.data.max_players).toBe(8)
      })

      it('should handle permission errors', async () => {
        const gameId = 666
        const updateData: UpdateGameRequest = { title: 'Unauthorized Update' }

        const permissionError = {
          response: {
            status: 403,
            data: { detail: 'Only the GM can update this game' },
          },
        }

        mockPut.mockRejectedValue(permissionError)

        await expect(apiClient.updateGame(gameId, updateData)).rejects.toEqual(permissionError)
      })

      it('should send dates in ISO 8601 format when updating (regression test)', async () => {
        // This test prevents regression of the bug where dates were sent in
        // "November 10, 2025 12:00 AM" format instead of ISO 8601 format
        const gameId = 1002
        const updateData: UpdateGameRequest = {
          title: 'Updated Game',
          description: 'Updated description',
          genre: 'Updated Genre',
          max_players: 6,
          start_date: '2025-12-01T18:00:00Z', // ISO 8601 format
          end_date: '2025-12-31T23:59:00Z',   // ISO 8601 format
          recruitment_deadline: '2025-11-25T23:59:00Z', // ISO 8601 format
          is_public: true,
          is_anonymous: false,
        }

        const mockUpdatedGame: Game = {
          id: gameId,
          ...updateData,
          gm_user_id: 1,
          state: 'setup',
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T20:00:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockUpdatedGame,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPut.mockResolvedValue(expectedResponse)

        await apiClient.updateGame(gameId, updateData)

        // Verify the API was called with properly formatted ISO 8601 dates
        expect(mockPut).toHaveBeenCalledWith(`/api/v1/games/${gameId}`, updateData)
        const callData = mockPut.mock.calls[0][1] as UpdateGameRequest

        // Verify dates are in ISO 8601 format (YYYY-MM-DDTHH:mm:ssZ)
        expect(callData.start_date).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
        expect(callData.end_date).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
        expect(callData.recruitment_deadline).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
      })
    })

    describe('deleteGame', () => {
      it('should delete a game', async () => {
        const gameId = 777

        const expectedResponse: AxiosResponse<void> = {
          data: undefined,
          status: 204,
          statusText: 'No Content',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockDelete.mockResolvedValue(expectedResponse)

        const result = await apiClient.deleteGame(gameId)

        expect(mockDelete).toHaveBeenCalledWith(`/api/v1/games/${gameId}`)
        expect(result).toEqual(expectedResponse)
      })

      it('should handle delete permission errors', async () => {
        const gameId = 888

        const permissionError = {
          response: {
            status: 403,
            data: { detail: 'Only the GM can delete this game' },
          },
        }

        mockDelete.mockRejectedValue(permissionError)

        await expect(apiClient.deleteGame(gameId)).rejects.toEqual(permissionError)
      })
    })

    describe('updateGameState', () => {
      it('should update game state', async () => {
        const gameId = 333
        const stateData: UpdateGameStateRequest = {
          state: 'recruiting'
        }

        const mockUpdatedGame: Game = {
          id: gameId,
          title: 'State Updated Game',
          description: 'A game with updated state',
          gm_user_id: 1,
          state: 'recruiting',
          genre: 'Action',
          max_players: 4,
          start_date: '2025-09-01T19:00:00Z',
          recruitment_deadline: '2025-08-25T23:59:59Z',
          is_public: true,
          current_phase_id: null,
          created_at: '2025-08-07T18:30:00Z',
          updated_at: '2025-08-07T19:30:00Z',
        }

        const expectedResponse: AxiosResponse<Game> = {
          data: mockUpdatedGame,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPut.mockResolvedValue(expectedResponse)

        const result = await apiClient.updateGameState(gameId, stateData)

        expect(mockPut).toHaveBeenCalledWith(`/api/v1/games/${gameId}/state`, stateData)
        expect(result).toEqual(expectedResponse)
        expect(result.data.state).toBe('recruiting')
      })

      it('should handle invalid state transitions', async () => {
        const gameId = 444
        const invalidStateData = {
          state: 'invalid_state'
        } as UpdateGameStateRequest

        const validationError = {
          response: {
            status: 422,
            data: { detail: 'Invalid state transition' },
          },
        }

        mockPut.mockRejectedValue(validationError)

        await expect(apiClient.updateGameState(gameId, invalidStateData)).rejects.toEqual(validationError)
      })
    })

    describe('leaveGame', () => {
      it('should allow player to leave game', async () => {
        const gameId = 111

        const expectedResponse: AxiosResponse<void> = {
          data: undefined,
          status: 204,
          statusText: 'No Content',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockDelete.mockResolvedValue(expectedResponse)

        const result = await apiClient.leaveGame(gameId)

        expect(mockDelete).toHaveBeenCalledWith(`/api/v1/games/${gameId}/leave`)
        expect(result).toEqual(expectedResponse)
      })

      it('should handle leaving game not participated in', async () => {
        const gameId = 222

        const notParticipantError = {
          response: {
            status: 400,
            data: { detail: 'You are not a participant in this game' },
          },
        }

        mockDelete.mockRejectedValue(notParticipantError)

        await expect(apiClient.leaveGame(gameId)).rejects.toEqual(notParticipantError)
      })
    })
  })

  describe('Game application endpoints', () => {
    describe('applyToGame', () => {
      it('should submit application to game', async () => {
        const gameId = 123
        const applicationData: ApplyToGameRequest = {
          role: 'player',
          message: 'I would love to join this campaign!',
        }

        const mockApplication: GameApplication = {
          id: 456,
          game_id: gameId,
          user_id: 789,
          username: 'applicant',
          role: 'player',
          message: 'I would love to join this campaign!',
          status: 'pending',
          response_message: null,
          applied_at: '2025-08-07T18:30:00Z',
          reviewed_at: null,
        }

        const expectedResponse: AxiosResponse<GameApplication> = {
          data: mockApplication,
          status: 201,
          statusText: 'Created',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPost.mockResolvedValue(expectedResponse)

        const result = await apiClient.applyToGame(gameId, applicationData)

        expect(mockPost).toHaveBeenCalledWith(`/api/v1/games/${gameId}/apply`, applicationData)
        expect(result).toEqual(expectedResponse)
        expect(result.data.status).toBe('pending')
      })

      it('should handle duplicate applications', async () => {
        const gameId = 123
        const applicationData: ApplyToGameRequest = {
          role: 'player',
        }

        const duplicateError = {
          response: {
            status: 409,
            data: { detail: 'You have already applied to this game' },
          },
        }

        mockPost.mockRejectedValue(duplicateError)

        await expect(apiClient.applyToGame(gameId, applicationData)).rejects.toEqual(duplicateError)
      })
    })

    describe('getGameApplications', () => {
      it('should fetch game applications for GM', async () => {
        const gameId = 123
        const mockApplications: GameApplication[] = [
          {
            id: 1,
            game_id: gameId,
            user_id: 100,
            username: 'player1',
            role: 'player',
            message: 'Excited to play!',
            status: 'pending',
            response_message: null,
            applied_at: '2025-08-07T18:30:00Z',
            reviewed_at: null,
          },
          {
            id: 2,
            game_id: gameId,
            user_id: 101,
            username: 'player2',
            role: 'audience',
            message: 'Would love to observe',
            status: 'approved',
            response_message: 'Welcome!',
            applied_at: '2025-08-06T10:00:00Z',
            reviewed_at: '2025-08-07T09:00:00Z',
          },
        ]

        const expectedResponse: AxiosResponse<GameApplication[]> = {
          data: mockApplications,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockGet.mockResolvedValue(expectedResponse)

        const result = await apiClient.getGameApplications(gameId)

        expect(mockGet).toHaveBeenCalledWith(`/api/v1/games/${gameId}/applications`)
        expect(result).toEqual(expectedResponse)
        expect(result.data).toHaveLength(2)
      })

      it('should handle GM access only', async () => {
        const gameId = 456

        const permissionError = {
          response: {
            status: 403,
            data: { detail: 'Only the GM can view applications' },
          },
        }

        mockGet.mockRejectedValue(permissionError)

        await expect(apiClient.getGameApplications(gameId)).rejects.toEqual(permissionError)
      })
    })

    describe('reviewGameApplication', () => {
      it('should approve application', async () => {
        const gameId = 123
        const applicationId = 456
        const reviewData: ReviewApplicationRequest = {
          status: 'approved',
          response_message: 'Welcome to the campaign!',
        }

        const mockReviewedApplication: GameApplication = {
          id: applicationId,
          game_id: gameId,
          user_id: 789,
          username: 'applicant',
          role: 'player',
          message: 'Original message',
          status: 'approved',
          response_message: 'Welcome to the campaign!',
          applied_at: '2025-08-07T18:30:00Z',
          reviewed_at: '2025-08-07T20:00:00Z',
        }

        const expectedResponse: AxiosResponse<GameApplication> = {
          data: mockReviewedApplication,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPut.mockResolvedValue(expectedResponse)

        const result = await apiClient.reviewGameApplication(gameId, applicationId, reviewData)

        expect(mockPut).toHaveBeenCalledWith(
          `/api/v1/games/${gameId}/applications/${applicationId}/review`,
          reviewData
        )
        expect(result).toEqual(expectedResponse)
        expect(result.data.status).toBe('approved')
        expect(result.data.reviewed_at).toBeTruthy()
      })

      it('should reject application', async () => {
        const gameId = 123
        const applicationId = 456
        const reviewData: ReviewApplicationRequest = {
          status: 'rejected',
          response_message: 'Sorry, the campaign is full.',
        }

        const mockRejectedApplication: GameApplication = {
          id: applicationId,
          game_id: gameId,
          user_id: 789,
          username: 'applicant',
          role: 'player',
          message: 'Original message',
          status: 'rejected',
          response_message: 'Sorry, the campaign is full.',
          applied_at: '2025-08-07T18:30:00Z',
          reviewed_at: '2025-08-07T20:00:00Z',
        }

        const expectedResponse: AxiosResponse<GameApplication> = {
          data: mockRejectedApplication,
          status: 200,
          statusText: 'OK',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockPut.mockResolvedValue(expectedResponse)

        const result = await apiClient.reviewGameApplication(gameId, applicationId, reviewData)

        expect(result.data.status).toBe('rejected')
      })
    })

    describe('withdrawGameApplication', () => {
      it('should withdraw application', async () => {
        const gameId = 789

        const expectedResponse: AxiosResponse<void> = {
          data: undefined,
          status: 204,
          statusText: 'No Content',
          headers: {},
          config: {} as InternalAxiosRequestConfig,
        }

        mockDelete.mockResolvedValue(expectedResponse)

        const result = await apiClient.withdrawGameApplication(gameId)

        expect(mockDelete).toHaveBeenCalledWith(`/api/v1/games/${gameId}/application`)
        expect(result).toEqual(expectedResponse)
      })

      it('should handle no application to withdraw', async () => {
        const gameId = 999

        const noApplicationError = {
          response: {
            status: 404,
            data: { detail: 'No application found for this game' },
          },
        }

        mockDelete.mockRejectedValue(noApplicationError)

        await expect(apiClient.withdrawGameApplication(gameId)).rejects.toEqual(noApplicationError)
      })
    })
  })

  describe('Error handling for all endpoints', () => {
    it('should handle network errors', async () => {
      const networkError = new Error('Network Error')
      networkError.code = 'ECONNREFUSED'

      mockGet.mockRejectedValue(networkError)

      await expect(apiClient.getAllGames()).rejects.toEqual(networkError)
    })

    it('should handle authentication errors', async () => {
      const authError = {
        response: {
          status: 401,
          data: { detail: 'Authentication required' },
        },
      }

      mockGet.mockRejectedValue(authError)

      await expect(apiClient.getAllGames()).rejects.toEqual(authError)
    })

    it('should handle server errors', async () => {
      const serverError = {
        response: {
          status: 500,
          data: { detail: 'Internal server error' },
        },
      }

      mockPost.mockRejectedValue(serverError)

      const createData: CreateGameRequest = {
        title: 'Test Game',
        description: 'Test Description',
        genre: 'Test Genre',
        max_players: 4,
        start_date: '2025-09-01T19:00:00Z',
        recruitment_deadline: '2025-08-25T23:59:59Z',
      }

      await expect(apiClient.createGame(createData)).rejects.toEqual(serverError)
    })
  })
})
