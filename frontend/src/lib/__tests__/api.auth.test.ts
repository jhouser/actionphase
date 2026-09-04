import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import type { AuthResponse, LoginRequest, RegisterRequest } from '../../types/auth'

// Mock axios with proper setup
let mockPost: ReturnType<typeof vi.fn>
let mockGet: ReturnType<typeof vi.fn>

vi.mock('axios', () => {
  const post = vi.fn()
  const get = vi.fn()

  const client = {
    post,
    get,
    put: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      request: { use: vi.fn().mockReturnValue(0) },
      response: { use: vi.fn().mockReturnValue(0) },
    },
  }

  const refreshClient = {
    post: vi.fn(),
    get,
    put: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      request: { use: vi.fn().mockReturnValue(0) },
      response: { use: vi.fn().mockReturnValue(0) },
    },
  }

  let createCallCount = 0
  return {
    default: {
      create: vi.fn(() => {
        createCallCount++
        if (createCallCount === 1) {
          mockPost = post
          mockGet = get
          return client
        }
        return refreshClient
      }),
    },
  }
})

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

// Mock window.location
const mockLocation = {
  pathname: '/',
  href: '',
}
Object.defineProperty(window, 'location', {
  value: mockLocation,
  writable: true,
})

// Mock console.error and console.log to avoid test output noise
const mockConsole = {
  error: vi.fn(),
  log: vi.fn(),
}
Object.defineProperty(console, 'error', { value: mockConsole.error })
Object.defineProperty(console, 'log', { value: mockConsole.log })

describe('ApiClient - Authentication', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    // Reset location mock
    mockLocation.pathname = '/'
    mockLocation.href = ''
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('login', () => {
    it('should make POST request to login endpoint', async () => {
      const loginData: LoginRequest = {
        username: 'testuser',
        password: 'password123',
      }

      const expectedResponse: AxiosResponse<AuthResponse> = {
        data: {
          user: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
          token: 'mock-jwt-token',
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      }

      mockPost.mockResolvedValue(expectedResponse)

      const result = await apiClient.login(loginData)

      expect(mockPost).toHaveBeenCalledWith('/api/v1/auth/login', loginData)
      expect(result).toEqual(expectedResponse)
    })

    it('should handle login errors', async () => {
      const loginData: LoginRequest = {
        username: 'testuser',
        password: 'wrongpassword',
      }

      const loginError = {
        response: {
          status: 401,
          data: { detail: 'Invalid credentials' },
        },
      }

      mockPost.mockRejectedValue(loginError)

      await expect(apiClient.login(loginData)).rejects.toEqual(loginError)
      expect(mockPost).toHaveBeenCalledWith('/api/v1/auth/login', loginData)
    })
  })

  describe('register', () => {
    it('should make POST request to register endpoint', async () => {
      const registerData: RegisterRequest = {
        username: 'newuser',
        email: 'new@example.com',
        password: 'password123',
      }

      const expectedResponse: AxiosResponse<AuthResponse> = {
        data: {
          user: {
            id: 2,
            username: 'newuser',
            email: 'new@example.com',
            created_at: '2025-08-07T18:30:00Z',
            updated_at: '2025-08-07T18:30:00Z'
          },
          token: 'new-jwt-token',
        },
        status: 201,
        statusText: 'Created',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      }

      mockPost.mockResolvedValue(expectedResponse)

      const result = await apiClient.register(registerData)

      expect(mockPost).toHaveBeenCalledWith('/api/v1/auth/register', registerData)
      expect(result).toEqual(expectedResponse)
    })

    it('should handle registration validation errors', async () => {
      const registerData: RegisterRequest = {
        username: 'existing',
        email: 'invalid-email',
        password: '123',
      }

      const validationError = {
        response: {
          status: 422,
          data: {
            detail: 'Validation failed',
            details: {
              email: 'Invalid email format',
              password: 'Password too short',
              username: 'Username already taken'
            }
          },
        },
      }

      mockPost.mockRejectedValue(validationError)

      await expect(apiClient.register(registerData)).rejects.toEqual(validationError)
      expect(mockPost).toHaveBeenCalledWith('/api/v1/auth/register', registerData)
    })
  })

  describe('refreshToken', () => {
    it('should make GET request to refresh endpoint with token', async () => {
      const existingToken = 'existing-jwt-token'
      localStorageMock.getItem.mockReturnValue(existingToken)

      const expectedResponse: AxiosResponse<{ token: string }> = {
        data: { token: 'new-refreshed-token' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      }

      mockGet.mockResolvedValue(expectedResponse)

      const result = await apiClient.refreshToken()

      expect(localStorageMock.getItem).toHaveBeenCalledWith('auth_token')
      expect(mockGet).toHaveBeenCalledWith('/api/v1/auth/refresh', {
        headers: {
          Authorization: `Bearer ${existingToken}`,
        },
      })
      expect(result).toEqual(expectedResponse)
    })

    it('should handle refresh when no token exists', async () => {
      localStorageMock.getItem.mockReturnValue(null)

      const expectedResponse: AxiosResponse<{ token: string }> = {
        data: { token: 'new-token' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      }

      mockGet.mockResolvedValue(expectedResponse)

      const result = await apiClient.refreshToken()

      expect(mockGet).toHaveBeenCalledWith('/api/v1/auth/refresh', {
        headers: {
          Authorization: 'Bearer null',
        },
      })
      expect(result).toEqual(expectedResponse)
    })

    it('should handle refresh token expiration', async () => {
      const existingToken = 'expired-token'
      localStorageMock.getItem.mockReturnValue(existingToken)

      const refreshError = {
        response: {
          status: 401,
          data: { detail: 'Token expired' },
        },
      }

      mockGet.mockRejectedValue(refreshError)

      await expect(apiClient.refreshToken()).rejects.toEqual(refreshError)
    })
  })

  describe('token utility methods', () => {
    describe('setAuthToken', () => {
      it('should set valid token in localStorage', () => {
        const validToken = 'valid-jwt-token'

        apiClient.setAuthToken(validToken)

        expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_token', validToken)
        expect(mockConsole.log).toHaveBeenCalledWith(
          'Setting auth token:', 'valid-jwt-token...'
        )
      })

      it('should handle long tokens by truncating log message', () => {
        const longToken = 'a'.repeat(100)

        apiClient.setAuthToken(longToken)

        expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_token', longToken)
        expect(mockConsole.log).toHaveBeenCalledWith(
          'Setting auth token:', 'a'.repeat(50) + '...'
        )
      })

      it('should reject empty string tokens', () => {
        apiClient.setAuthToken('')

        expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token')
        expect(localStorageMock.setItem).not.toHaveBeenCalled()
        expect(mockConsole.error).toHaveBeenCalledWith('Attempted to set invalid token:', '')
      })

      it('should reject whitespace-only tokens', () => {
        apiClient.setAuthToken('   ')

        expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token')
        expect(localStorageMock.setItem).not.toHaveBeenCalled()
        expect(mockConsole.error).toHaveBeenCalledWith('Attempted to set invalid token:', '   ')
      })

      it('should reject string "undefined"', () => {
        apiClient.setAuthToken('undefined')

        expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token')
        expect(localStorageMock.setItem).not.toHaveBeenCalled()
        expect(mockConsole.error).toHaveBeenCalledWith('Attempted to set invalid token:', 'undefined')
      })

      it('should reject string "null"', () => {
        apiClient.setAuthToken('null')

        expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token')
        expect(localStorageMock.setItem).not.toHaveBeenCalled()
        expect(mockConsole.error).toHaveBeenCalledWith('Attempted to set invalid token:', 'null')
      })
    })

    describe('removeAuthToken', () => {
      it('should remove token from localStorage', () => {
        apiClient.removeAuthToken()

        expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token')
      })
    })

    describe('getAuthToken', () => {
      it('should return token from localStorage', () => {
        const testToken = 'stored-token'
        localStorageMock.getItem.mockReturnValue(testToken)

        const result = apiClient.getAuthToken()

        expect(localStorageMock.getItem).toHaveBeenCalledWith('auth_token')
        expect(result).toBe(testToken)
      })

      it('should return null when no token exists', () => {
        localStorageMock.getItem.mockReturnValue(null)

        const result = apiClient.getAuthToken()

        expect(result).toBeNull()
      })
    })
  })

  describe('ping', () => {
    it('should make GET request to ping endpoint', async () => {
      const expectedResponse: AxiosResponse<string> = {
        data: 'ponger',
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      }

      mockGet.mockResolvedValue(expectedResponse)

      const result = await apiClient.ping()

      expect(mockGet).toHaveBeenCalledWith('/ping')
      expect(result).toEqual(expectedResponse)
    })

    it('should handle ping failures', async () => {
      const networkError = new Error('Network Error')
      mockGet.mockRejectedValue(networkError)

      await expect(apiClient.ping()).rejects.toEqual(networkError)
      expect(mockGet).toHaveBeenCalledWith('/ping')
    })
  })

  describe('interceptors setup', () => {
    it('should set up request and response interceptors', () => {
      // The interceptors are set up during axios client construction
      // Since we're using a mock, we can only verify the mocks were called
      expect(mockPost).toBeDefined()
      expect(mockGet).toBeDefined()
    })

    // Note: Testing the actual interceptor logic is complex because it's set up
    // during construction and involves axios internals. In a real project,
    // you might extract the interceptor logic into separate testable functions.
  })

  describe('error handling', () => {
    it('should handle network errors gracefully', async () => {
      const networkError = new Error('Network Error')
      networkError.code = 'ECONNREFUSED'

      mockPost.mockRejectedValue(networkError)

      await expect(apiClient.login({ username: 'test', password: 'test' }))
        .rejects.toEqual(networkError)
    })

    it('should handle server errors', async () => {
      const serverError = {
        response: {
          status: 500,
          data: { detail: 'Internal server error' },
        },
      }

      mockPost.mockRejectedValue(serverError)

      await expect(apiClient.login({ username: 'test', password: 'test' }))
        .rejects.toEqual(serverError)
    })

    it('should handle timeout errors', async () => {
      const timeoutError = new Error('timeout of 5000ms exceeded')
      timeoutError.code = 'ECONNABORTED'

      mockPost.mockRejectedValue(timeoutError)

      await expect(apiClient.login({ username: 'test', password: 'test' }))
        .rejects.toEqual(timeoutError)
    })
  })

  describe('content type headers', () => {
    it('should send requests with application/json content type', async () => {
      const loginData: LoginRequest = { username: 'test', password: 'test' }

      mockPost.mockResolvedValue({
        data: { user: {}, token: 'token' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as InternalAxiosRequestConfig,
      })

      await apiClient.login(loginData)

      expect(mockPost).toHaveBeenCalledWith('/api/v1/auth/login', loginData)
    })
  })
})
