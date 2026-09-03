import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { CommunityBannerSection } from '../CommunityBannerSection'
import type { Community } from '../../../types/communities'

const uploadCommunityBanner = vi.fn()
const deleteCommunityBanner = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      uploadCommunityBanner: (slug: string, file: File) =>
        uploadCommunityBanner(slug, file),
      deleteCommunityBanner: (slug: string) => deleteCommunityBanner(slug),
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
  description: 'We fly at dusk',
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: 'moderator',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const withBanner: Community = {
  ...community,
  banner_url: 'http://localhost:3000/uploads/banners/communities/1/123.png',
}

// jsdom implements neither, and the component depends on both to build and
// release previews. Object URLs are counted so the revoke assertions below are
// about real calls rather than a no-op stub.
const createdUrls: string[] = []
const revokedUrls: string[] = []

beforeEach(() => {
  vi.clearAllMocks()
  createdUrls.length = 0
  revokedUrls.length = 0

  let counter = 0
  globalThis.URL.createObjectURL = vi.fn(() => {
    const url = `blob:mock-${++counter}`
    createdUrls.push(url)
    return url
  })
  globalThis.URL.revokeObjectURL = vi.fn((url: string) => {
    revokedUrls.push(url)
  })
})

function pickFile(name = 'banner.png', type = 'image/png') {
  const input = screen.getByTestId('community-banner-input')
  const file = new File(['image-bytes'], name, { type })
  fireEvent.change(input, { target: { files: [file] } })
  return file
}

describe('CommunityBannerSection', () => {
  it('offers upload when there is no banner, and no remove control', () => {
    renderWithProviders(<CommunityBannerSection community={community} />)

    expect(screen.getByTestId('community-banner-choose')).toHaveTextContent('Upload banner')
    // Nothing to remove: offering it would produce a no-op button.
    expect(screen.queryByTestId('community-banner-remove')).not.toBeInTheDocument()
    expect(screen.queryByTestId('community-banner-preview')).not.toBeInTheDocument()
  })

  it('shows the existing banner and offers replace and remove', () => {
    renderWithProviders(<CommunityBannerSection community={withBanner} />)

    expect(screen.getByTestId('community-banner-preview')).toHaveAttribute(
      'src',
      withBanner.banner_url,
    )
    expect(screen.getByTestId('community-banner-choose')).toHaveTextContent('Replace banner')
    expect(screen.getByTestId('community-banner-remove')).toBeInTheDocument()
  })

  // The point of the preview-then-confirm flow: selecting a file must NOT
  // upload it. The server crops to 6:1, so the moderator sees the crop while
  // the current banner is still in place.
  it('previews a selected file without uploading it', async () => {
    renderWithProviders(<CommunityBannerSection community={withBanner} />)

    pickFile()

    await waitFor(() => {
      expect(screen.getByTestId('community-banner-preview')).toHaveAttribute('src', 'blob:mock-1')
    })
    expect(uploadCommunityBanner).not.toHaveBeenCalled()
    expect(screen.getByTestId('community-banner-confirm')).toBeInTheDocument()
  })

  it('uploads the pending file on confirm', async () => {
    uploadCommunityBanner.mockResolvedValue({ data: withBanner })
    renderWithProviders(<CommunityBannerSection community={community} />)

    const file = pickFile()
    fireEvent.click(await screen.findByTestId('community-banner-confirm'))

    await waitFor(() => {
      expect(uploadCommunityBanner).toHaveBeenCalledWith('midnight-ravens', file)
    })
    expect(showSuccess).toHaveBeenCalledWith('Banner updated')
  })

  it('discards a preview without uploading', async () => {
    renderWithProviders(<CommunityBannerSection community={withBanner} />)

    pickFile()
    fireEvent.click(await screen.findByTestId('community-banner-discard'))

    await waitFor(() => {
      // Back to the stored banner, not the blob.
      expect(screen.getByTestId('community-banner-preview')).toHaveAttribute(
        'src',
        withBanner.banner_url,
      )
    })
    expect(uploadCommunityBanner).not.toHaveBeenCalled()
    expect(revokedUrls).toContain('blob:mock-1')
  })

  // Every object URL must be released. Without this each preview leaks a blob
  // for the lifetime of the page, which no visible behaviour would reveal.
  it('revokes the previous object URL when a second file is chosen', async () => {
    renderWithProviders(<CommunityBannerSection community={community} />)

    pickFile('one.png')
    await screen.findByTestId('community-banner-confirm')
    pickFile('two.png')

    await waitFor(() => {
      expect(screen.getByTestId('community-banner-preview')).toHaveAttribute('src', 'blob:mock-2')
    })
    expect(revokedUrls).toContain('blob:mock-1')
  })

  it('revokes the object URL on unmount', async () => {
    const { unmount } = renderWithProviders(
      <CommunityBannerSection community={community} />,
    )

    pickFile()
    await screen.findByTestId('community-banner-confirm')
    unmount()

    await waitFor(() => expect(revokedUrls).toContain('blob:mock-1'))
  })

  it('removes an existing banner', async () => {
    deleteCommunityBanner.mockResolvedValue({ data: undefined })
    renderWithProviders(<CommunityBannerSection community={withBanner} />)

    fireEvent.click(screen.getByTestId('community-banner-remove'))

    await waitFor(() => {
      expect(deleteCommunityBanner).toHaveBeenCalledWith('midnight-ravens')
    })
    expect(showSuccess).toHaveBeenCalledWith('Banner removed')
  })

  // The server's message is the useful one -- it names the actual reason
  // ("image too large", "invalid file type") rather than a generic failure.
  it('surfaces the server error message on a failed upload', async () => {
    uploadCommunityBanner.mockRejectedValue({
      response: { data: { error: 'image too large. Maximum size is 5MB' } },
    })
    renderWithProviders(<CommunityBannerSection community={community} />)

    pickFile()
    fireEvent.click(await screen.findByTestId('community-banner-confirm'))

    await waitFor(() => {
      expect(showError).toHaveBeenCalledWith('image too large. Maximum size is 5MB')
    })
    expect(showSuccess).not.toHaveBeenCalled()
  })

  // A failed upload keeps the pending file so the same image can be retried
  // without picking it again.
  it('keeps the preview after a failed upload', async () => {
    uploadCommunityBanner.mockRejectedValue({ response: { data: {} } })
    renderWithProviders(<CommunityBannerSection community={community} />)

    pickFile()
    fireEvent.click(await screen.findByTestId('community-banner-confirm'))

    await waitFor(() => expect(showError).toHaveBeenCalled())
    expect(screen.getByTestId('community-banner-confirm')).toBeInTheDocument()
    expect(screen.getByTestId('community-banner-preview')).toHaveAttribute('src', 'blob:mock-1')
  })

  it('falls back to a generic message when the server sends none', async () => {
    deleteCommunityBanner.mockRejectedValue(new Error('network'))
    renderWithProviders(<CommunityBannerSection community={withBanner} />)

    fireEvent.click(screen.getByTestId('community-banner-remove'))

    await waitFor(() => {
      expect(showError).toHaveBeenCalledWith('Could not remove the banner')
    })
  })

  // The picker must not offer a type the server rejects.
  it('restricts the file picker to the server allowlist', () => {
    renderWithProviders(<CommunityBannerSection community={community} />)

    expect(screen.getByTestId('community-banner-input')).toHaveAttribute(
      'accept',
      'image/jpeg,image/png,image/webp',
    )
  })
})
