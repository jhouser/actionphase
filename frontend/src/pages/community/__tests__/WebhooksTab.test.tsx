import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { WebhooksTab } from '../WebhooksTab'
import type { Community, CommunityWebhook } from '../../../types/communities'

const listWebhooks = vi.fn()
const createWebhook = vi.fn()
const updateWebhook = vi.fn()
const deleteWebhook = vi.fn()
const testWebhook = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listWebhooks: (slug: string) => listWebhooks(slug),
      createWebhook: (slug: string, data: unknown) => createWebhook(slug, data),
      updateWebhook: (slug: string, id: number, data: unknown) =>
        updateWebhook(slug, id, data),
      deleteWebhook: (slug: string, id: number) => deleteWebhook(slug, id),
      testWebhook: (slug: string, id: number) => testWebhook(slug, id),
    },
  },
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('../../../contexts/ToastContext', async () => {
  const actual = await vi.importActual('../../../contexts/ToastContext')
  return { ...actual, useToast: () => ({ showSuccess, showError }) }
})

const community: Community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: 'moderator',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

// The URL the SERVER returns: already masked. The client never sees any other
// form, which is the premise the whole component is built around.
const MASKED_URL = 'https://discord.com/api/webhooks/998877/••••cd34'

const healthyHook: CommunityWebhook = {
  id: 5,
  community_id: 1,
  url: MASKED_URL,
  label: '#announcements',
  is_enabled: true,
  events: ['recruitment', 'completed'],
  last_success_at: '2026-09-01T10:00:00Z',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-09-01T10:00:00Z',
}

const failingHook: CommunityWebhook = {
  ...healthyHook,
  id: 6,
  label: '#recruitment',
  last_success_at: '2026-08-20T09:00:00Z',
  last_error: 'discord webhook: HTTP 404: Unknown Webhook',
  last_error_at: '2026-09-01T14:00:00Z',
}

const unusedHook: CommunityWebhook = {
  id: 7,
  community_id: 1,
  url: MASKED_URL,
  is_enabled: true,
  events: [],
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const renderTab = (canModerate = true) =>
  renderWithProviders(<WebhooksTab community={community} canModerate={canModerate} />)

beforeEach(() => {
  vi.clearAllMocks()
  listWebhooks.mockResolvedValue({ data: [healthyHook] })
  createWebhook.mockResolvedValue({ data: healthyHook })
  updateWebhook.mockResolvedValue({ data: healthyHook })
  deleteWebhook.mockResolvedValue({ data: undefined })
  testWebhook.mockResolvedValue({ data: { success: true, message: 'ok' } })
})

describe('WebhooksTab', () => {
  describe('credential handling', () => {
    /**
     * The defining test of this component.
     *
     * The client only ever holds a MASKED url. Pre-filling the edit form with
     * it and submitting would overwrite the stored credential with bullet
     * characters -- silently breaking delivery while the UI reported success.
     */
    it('does not prefill the edit form with the masked URL', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-edit'))

      const urlInput = screen.getByTestId('webhook-url-input') as HTMLInputElement
      expect(urlInput.value).toBe('')
      expect(urlInput.value).not.toContain('••••')
    })

    it('omits url entirely when saving an edit the moderator did not retype', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-edit'))

      fireEvent.change(screen.getByTestId('webhook-label-input'), {
        target: { value: '#general' },
      })
      fireEvent.click(screen.getByTestId('webhook-save'))

      await waitFor(() => expect(updateWebhook).toHaveBeenCalled())

      const [, , payload] = updateWebhook.mock.calls[0]
      expect(payload).not.toHaveProperty('url')
      expect(payload.label).toBe('#general')
    })

    it('sends url only when the moderator types a replacement', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-edit'))

      const rotated = 'https://discord.com/api/webhooks/111/NewTokenValue'
      fireEvent.change(screen.getByTestId('webhook-url-input'), {
        target: { value: rotated },
      })
      fireEvent.click(screen.getByTestId('webhook-save'))

      await waitFor(() => expect(updateWebhook).toHaveBeenCalled())
      expect(updateWebhook.mock.calls[0][2].url).toBe(rotated)
    })

    it('never sends the masked URL back to the server on a status toggle', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-toggle'))

      await waitFor(() => expect(updateWebhook).toHaveBeenCalled())

      const payload = updateWebhook.mock.calls[0][2]
      expect(payload).not.toHaveProperty('url')
      expect(JSON.stringify(payload)).not.toContain('••••')
    })
  })

  describe('delivery status', () => {
    // Three distinct states, and collapsing any two would mislead a moderator
    // about whether their channel is working.
    it('shows a never-used webhook as unused rather than as a failure', async () => {
      listWebhooks.mockResolvedValue({ data: [unusedHook] })
      renderTab()

      expect(await screen.findByTestId('webhook-status-unused')).toBeInTheDocument()
      expect(screen.queryByTestId('webhook-last-error')).not.toBeInTheDocument()
    })

    it('surfaces the error text for a failing webhook', async () => {
      listWebhooks.mockResolvedValue({ data: [failingHook] })
      renderTab()

      expect(await screen.findByTestId('webhook-last-error')).toHaveTextContent(
        'Unknown Webhook'
      )
    })

    // The case the last_* columns exist for: a moderator needs to see that it
    // used to work, or they cannot tell a broken URL from one never configured.
    it('shows both the last success and the current failure', async () => {
      listWebhooks.mockResolvedValue({ data: [failingHook] })
      renderTab()

      const status = await screen.findByTestId('webhook-status')
      expect(status).toHaveTextContent(/Last failed/)
      expect(status).toHaveTextContent(/Last delivered successfully/)
    })

    it('warns when a webhook is subscribed to no events', async () => {
      listWebhooks.mockResolvedValue({ data: [unusedHook] })
      renderTab()

      expect(await screen.findByTestId('webhook-events')).toHaveTextContent(
        /never fire/
      )
    })
  })

  describe('create', () => {
    it('sends the typed URL and selected events', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('add-webhook'))

      const url = 'https://discord.com/api/webhooks/222/FreshToken'
      fireEvent.change(screen.getByTestId('webhook-url-input'), {
        target: { value: url },
      })
      fireEvent.click(screen.getByTestId('webhook-event-recruitment'))
      fireEvent.click(screen.getByTestId('webhook-save'))

      await waitFor(() => expect(createWebhook).toHaveBeenCalled())

      const payload = createWebhook.mock.calls[0][1]
      expect(payload.url).toBe(url)
      expect(payload.events).toEqual(['recruitment'])
    })

    it('reports the server error when the URL is rejected', async () => {
      createWebhook.mockRejectedValue({
        response: { data: { error: 'webhook URL must be an https Discord webhook endpoint' } },
      })
      renderTab()

      fireEvent.click(await screen.findByTestId('add-webhook'))
      fireEvent.change(screen.getByTestId('webhook-url-input'), {
        target: { value: 'https://evil.test/api/webhooks/1/t' },
      })
      fireEvent.click(screen.getByTestId('webhook-save'))

      await waitFor(() =>
        expect(showError).toHaveBeenCalledWith(
          expect.stringContaining('Discord webhook endpoint')
        )
      )
    })
  })

  describe('delete', () => {
    it('does not delete until the moderator confirms', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-delete'))

      // Asserted after a flush, not synchronously. A delete fired ALONGSIDE
      // opening the prompt would still be pending at this point, so an
      // immediate assertion passes against exactly the bug it should catch --
      // confirmed by mutation.
      await waitFor(() => {
        expect(screen.getByTestId('confirm-modal-message')).toBeInTheDocument()
      })
      expect(deleteWebhook).not.toHaveBeenCalled()
      expect(screen.getByTestId('confirm-modal-message')).toHaveTextContent(
        '#announcements'
      )
    })

    it('deletes once confirmed', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-delete'))
      fireEvent.click(screen.getByTestId('confirm-modal-confirm'))

      await waitFor(() => expect(deleteWebhook).toHaveBeenCalledWith('midnight-ravens', 5))
    })

    it('does not delete when cancelled', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-delete'))
      fireEvent.click(screen.getByTestId('confirm-modal-cancel'))

      expect(deleteWebhook).not.toHaveBeenCalled()
    })
  })

  describe('test button', () => {
    it('reports success', async () => {
      renderTab()
      fireEvent.click(await screen.findByTestId('webhook-test'))

      await waitFor(() => expect(testWebhook).toHaveBeenCalledWith('midnight-ravens', 5))
      await waitFor(() =>
        expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('delivered'))
      )
    })

    // The reason IS the feature -- a moderator presses this to learn why
    // delivery is failing, so a generic message would defeat the point.
    it("surfaces Discord's reason on failure", async () => {
      testWebhook.mockRejectedValue({
        response: { data: { error: 'webhook test failed: HTTP 404: Unknown Webhook' } },
      })
      renderTab()

      fireEvent.click(await screen.findByTestId('webhook-test'))
      await waitFor(() =>
        expect(showError).toHaveBeenCalledWith(
          expect.stringContaining('Unknown Webhook')
        )
      )
    })
  })

  describe('permissions', () => {
    it('renders nothing manageable for a non-moderator', () => {
      renderTab(false)

      expect(screen.queryByTestId('add-webhook')).not.toBeInTheDocument()
      expect(screen.getByText(/only this community's moderators/i)).toBeInTheDocument()
    })
  })
})
