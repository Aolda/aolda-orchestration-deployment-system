import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../auth/oidc', () => ({
  clearEmergencyAuthSession: vi.fn(),
  clearOIDCSession: vi.fn(),
  ensureOIDCAccessToken: vi.fn(async () => undefined),
  hasEmergencyAuthSession: vi.fn(() => false),
  isOIDCAuthEnabled: vi.fn(() => false),
}))

import { api } from './client'

describe('api client timeouts', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('일반 API 요청이 오래 멈추면 timeout 에러로 종료한다', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
        return new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener(
            'abort',
            () => {
              reject(new DOMException('Aborted', 'AbortError'))
            },
            { once: true },
          )
        })
      }),
    )

    const requestPromise = api.getProjects()
    const capturedError = requestPromise.catch((error) => error)
    await vi.advanceTimersByTimeAsync(15_000)

    await expect(capturedError).resolves.toMatchObject({
      code: 'REQUEST_TIMEOUT',
    })
  })

  it('백엔드 연결 자체가 실패하면 network 에러로 변환한다', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch')
      }),
    )

    await expect(api.getProjects()).rejects.toMatchObject({
      code: 'NETWORK_ERROR',
    })
  })

  it('로그 스트림 API 오류의 details를 보존한다', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            error: {
              code: 'INTEGRATION_ERROR',
              message: 'An unexpected integration error occurred.',
              details: {
                error: 'kubernetes api /api/v1/namespaces/shared/pods/missing/log failed with status 404',
              },
            },
          }),
          {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      ),
    )

    await expect(
      api.streamApplicationLogs('shared__moltbot-front-poc-web', {
        podName: 'missing',
        containerName: 'web',
        onEvent: vi.fn(),
      }),
    ).rejects.toMatchObject({
      code: 'INTEGRATION_ERROR',
      details: {
        error: 'kubernetes api /api/v1/namespaces/shared/pods/missing/log failed with status 404',
      },
    })
  })
})
