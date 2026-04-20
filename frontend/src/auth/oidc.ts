const authMode = import.meta.env.VITE_AODS_AUTH_MODE ?? 'header'
const issuerUrl = import.meta.env.VITE_AODS_OIDC_ISSUER_URL ?? ''
const clientId = import.meta.env.VITE_AODS_OIDC_CLIENT_ID ?? ''
const scope = import.meta.env.VITE_AODS_OIDC_SCOPE ?? 'openid profile email'
const redirectUriEnv = import.meta.env.VITE_AODS_OIDC_REDIRECT_URI ?? ''
const emergencyLoginEnabled = (import.meta.env.VITE_AODS_ALLOW_EMERGENCY_LOGIN ?? 'false') === 'true'
const emergencyUsername = 'admin'
const emergencyPassword = 'qwe1356@'
const oidcRequestTimeoutMs = Number(import.meta.env.VITE_AODS_OIDC_REQUEST_TIMEOUT_MS ?? '15000')

const storageKeyPrefix = 'aods.oidc'
const accessTokenStorageKey = `${storageKeyPrefix}.access_token`
const idTokenStorageKey = `${storageKeyPrefix}.id_token`
const refreshTokenStorageKey = `${storageKeyPrefix}.refresh_token`
const expiresAtStorageKey = `${storageKeyPrefix}.expires_at`
const stateStorageKey = `${storageKeyPrefix}.state`
const verifierStorageKey = `${storageKeyPrefix}.code_verifier`
const emergencySessionStorageKey = `${storageKeyPrefix}.emergency_session`

type OIDCDiscoveryDocument = {
  authorization_endpoint: string
  token_endpoint: string
  end_session_endpoint?: string
}

type TokenResponse = {
  access_token: string
  id_token?: string
  refresh_token?: string
  expires_in?: number
  refresh_expires_in?: number
}

let discoveryPromise: Promise<OIDCDiscoveryDocument> | null = null
let accessTokenPromise: Promise<string | undefined> | null = null

export function isOIDCAuthEnabled() {
  return authMode === 'oidc'
}

export function isEmergencyLoginEnabled() {
  return emergencyLoginEnabled
}

export function hasEmergencyAuthSession() {
  if (!isOIDCAuthEnabled() || !emergencyLoginEnabled || typeof window === 'undefined') {
    return false
  }

  return window.sessionStorage.getItem(emergencySessionStorageKey) === 'true'
}

export function startEmergencyAuthSession(username: string, password: string) {
  if (!emergencyLoginEnabled || typeof window === 'undefined') {
    throw new Error('비상 로그인은 현재 비활성화되어 있습니다.')
  }
  if (username.trim() !== emergencyUsername || password !== emergencyPassword) {
    throw new Error('아이디 또는 비밀번호가 올바르지 않습니다.')
  }

  clearOIDCSession()
  window.sessionStorage.setItem(emergencySessionStorageKey, 'true')
}

export function clearEmergencyAuthSession() {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.removeItem(emergencySessionStorageKey)
}

export function hasOIDCAuthorizationCallback() {
  if (!isOIDCAuthEnabled() || typeof window === 'undefined') {
    return false
  }

  const url = new URL(window.location.href)
  return Boolean(url.searchParams.get('code') && url.searchParams.get('state'))
}

export function hasStoredOIDCSession() {
  if (!isOIDCAuthEnabled() || typeof window === 'undefined') {
    return false
  }

  return Boolean(readStoredAccessToken() || window.sessionStorage.getItem(refreshTokenStorageKey))
}

export function shouldResumeOIDCSession() {
  return isOIDCAuthEnabled() && !hasEmergencyAuthSession() && (hasOIDCAuthorizationCallback() || hasStoredOIDCSession())
}

export async function ensureOIDCAccessToken() {
  if (!isOIDCAuthEnabled() || hasEmergencyAuthSession()) {
    return undefined
  }

  const cachedToken = readStoredAccessToken()
  if (cachedToken) {
    return cachedToken
  }

  if (!accessTokenPromise) {
    accessTokenPromise = bootstrapOIDCFlow().finally(() => {
      accessTokenPromise = null
    })
  }

  return accessTokenPromise
}

export function clearOIDCSession() {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.removeItem(accessTokenStorageKey)
  window.sessionStorage.removeItem(idTokenStorageKey)
  window.sessionStorage.removeItem(refreshTokenStorageKey)
  window.sessionStorage.removeItem(expiresAtStorageKey)
  window.sessionStorage.removeItem(stateStorageKey)
  window.sessionStorage.removeItem(verifierStorageKey)
}

export async function logoutOIDCSession() {
  const idToken = readStoredIDToken()
  clearOIDCSession()

  if (!isOIDCAuthEnabled() || typeof window === 'undefined') {
    return
  }

  try {
    const discovery = await getOIDCDiscovery()
    if (!discovery.end_session_endpoint) {
      return
    }

    const logoutURL = new URL(discovery.end_session_endpoint)
    if (idToken) {
      logoutURL.searchParams.set('id_token_hint', idToken)
    } else {
      logoutURL.searchParams.set('client_id', clientId)
    }
    logoutURL.searchParams.set('post_logout_redirect_uri', redirectUri())
    window.location.assign(logoutURL.toString())
    return new Promise<never>(() => {})
  } catch {
    return
  }
}

function readStoredAccessToken() {
  if (typeof window === 'undefined') {
    return undefined
  }

  const token = window.sessionStorage.getItem(accessTokenStorageKey)
  if (!token) {
    return undefined
  }

  const expiresAt = Number(window.sessionStorage.getItem(expiresAtStorageKey) ?? '0')
  if (expiresAt > Date.now() + 30_000) {
    return token
  }

  return undefined
}

function readStoredIDToken() {
  if (typeof window === 'undefined') {
    return undefined
  }

  return window.sessionStorage.getItem(idTokenStorageKey) ?? undefined
}

async function bootstrapOIDCFlow(): Promise<string | undefined> {
  assertOIDCConfig()

  const url = new URL(window.location.href)
  const code = url.searchParams.get('code')
  const state = url.searchParams.get('state')

  if (code && state) {
    return exchangeAuthorizationCode(code, state)
  }

  const refreshed = await refreshAccessToken()
  if (refreshed) {
    return refreshed
  }

  await startAuthorizationRedirect()
  return undefined
}

async function refreshAccessToken() {
  const refreshToken = window.sessionStorage.getItem(refreshTokenStorageKey)
  if (!refreshToken) {
    return undefined
  }

  const discovery = await getOIDCDiscovery()
  const response = await fetchWithTimeout(discovery.token_endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: new URLSearchParams({
      grant_type: 'refresh_token',
      client_id: clientId,
      refresh_token: refreshToken,
    }),
  })

  if (!response.ok) {
    clearOIDCSession()
    return undefined
  }

  const payload = (await response.json()) as TokenResponse
  persistTokens(payload)
  return payload.access_token
}

async function exchangeAuthorizationCode(code: string, returnedState: string) {
  const expectedState = window.sessionStorage.getItem(stateStorageKey)
  const codeVerifier = window.sessionStorage.getItem(verifierStorageKey)
  if (!expectedState || expectedState !== returnedState || !codeVerifier) {
    clearOIDCSession()
    throw new Error('OIDC state validation failed.')
  }

  const discovery = await getOIDCDiscovery()
  const response = await fetchWithTimeout(discovery.token_endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      client_id: clientId,
      code,
      redirect_uri: redirectUri(),
      code_verifier: codeVerifier,
    }),
  })

  if (!response.ok) {
    clearOIDCSession()
    throw new Error(`OIDC token exchange failed with ${response.status}.`)
  }

  const payload = (await response.json()) as TokenResponse
  persistTokens(payload)
  clearAuthorizationCallbackQuery()
  return payload.access_token
}

async function startAuthorizationRedirect() {
  const discovery = await getOIDCDiscovery()
  const codeVerifier = randomString(64)
  const state = randomString(32)
  const codeChallenge = await pkceChallenge(codeVerifier)

  window.sessionStorage.setItem(stateStorageKey, state)
  window.sessionStorage.setItem(verifierStorageKey, codeVerifier)

  const authorizationURL = new URL(discovery.authorization_endpoint)
  authorizationURL.searchParams.set('client_id', clientId)
  authorizationURL.searchParams.set('redirect_uri', redirectUri())
  authorizationURL.searchParams.set('response_type', 'code')
  authorizationURL.searchParams.set('scope', scope)
  authorizationURL.searchParams.set('state', state)
  authorizationURL.searchParams.set('code_challenge', codeChallenge)
  authorizationURL.searchParams.set('code_challenge_method', 'S256')

  window.location.assign(authorizationURL.toString())
  return new Promise<never>(() => {})
}

async function getOIDCDiscovery() {
  if (!discoveryPromise) {
    const discoveryURL = `${issuerUrl.replace(/\/$/, '')}/.well-known/openid-configuration`
    discoveryPromise = fetchWithTimeout(discoveryURL)
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`OIDC discovery failed with ${response.status}.`)
        }
        return (await response.json()) as OIDCDiscoveryDocument
      })
      .catch((error) => {
        discoveryPromise = null
        throw error
      })
  }

  return discoveryPromise
}

function persistTokens(payload: TokenResponse) {
  if (!payload.access_token) {
    throw new Error('OIDC token response did not include access_token.')
  }

  window.sessionStorage.setItem(accessTokenStorageKey, payload.access_token)
  if (payload.id_token) {
    window.sessionStorage.setItem(idTokenStorageKey, payload.id_token)
  }
  if (payload.refresh_token) {
    window.sessionStorage.setItem(refreshTokenStorageKey, payload.refresh_token)
  } else {
    window.sessionStorage.removeItem(refreshTokenStorageKey)
  }

  const expiresAt = Date.now() + Math.max(payload.expires_in ?? 0, 60) * 1000
  window.sessionStorage.setItem(expiresAtStorageKey, String(expiresAt))
  window.sessionStorage.removeItem(stateStorageKey)
  window.sessionStorage.removeItem(verifierStorageKey)
}

function clearAuthorizationCallbackQuery() {
  const url = new URL(window.location.href)
  url.searchParams.delete('code')
  url.searchParams.delete('state')
  url.searchParams.delete('session_state')
  url.searchParams.delete('iss')
  window.history.replaceState({}, document.title, url.toString())
}

function redirectUri() {
  if (redirectUriEnv) {
    assertRedirectOriginMatchesCurrentLocation(redirectUriEnv)
    return redirectUriEnv
  }
  return `${window.location.origin}${window.location.pathname}`
}

function assertOIDCConfig() {
  if (!issuerUrl) {
    throw new Error('VITE_AODS_OIDC_ISSUER_URL is required when VITE_AODS_AUTH_MODE=oidc.')
  }
  if (!clientId) {
    throw new Error('VITE_AODS_OIDC_CLIENT_ID is required when VITE_AODS_AUTH_MODE=oidc.')
  }
}

async function fetchWithTimeout(input: RequestInfo | URL, init?: RequestInit) {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), oidcRequestTimeoutMs)

  try {
    return await fetch(input, {
      ...init,
      signal: init?.signal ?? controller.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new Error(`OIDC 요청 시간이 초과되었습니다. ${Math.round(oidcRequestTimeoutMs / 1000)}초 후 중단했습니다.`)
    }
    throw error
  } finally {
    window.clearTimeout(timeout)
  }
}

function assertRedirectOriginMatchesCurrentLocation(configuredRedirectUri: string) {
  if (typeof window === 'undefined') {
    return
  }

  try {
    const configuredURL = new URL(configuredRedirectUri, window.location.href)
    const currentURL = new URL(window.location.href)
    if (
      isLoopbackOrigin(configuredURL.origin) &&
      isLoopbackOrigin(currentURL.origin) &&
      configuredURL.origin !== currentURL.origin
    ) {
      throw new Error(
        `OIDC redirect URI가 현재 프론트 주소(${currentURL.origin})와 다릅니다. 현재 설정은 ${configuredURL.origin}으로 고정되어 있습니다. 프론트를 5173으로 띄우거나 VITE_AODS_OIDC_REDIRECT_URI를 현재 origin으로 맞추세요.`,
      )
    }
  } catch (error) {
    if (error instanceof Error) {
      throw error
    }
    throw new Error('OIDC redirect URI 형식이 올바르지 않습니다.')
  }
}

function isLoopbackOrigin(origin: string) {
  try {
    const parsed = new URL(origin)
    return parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1'
  } catch {
    return false
  }
}

function randomString(length: number) {
  const bytes = new Uint8Array(length)
  window.crypto.getRandomValues(bytes)
  return base64UrlEncode(bytes).slice(0, length)
}

async function pkceChallenge(value: string) {
  const encoded = new TextEncoder().encode(value)
  const digest = await window.crypto.subtle.digest('SHA-256', encoded)
  return base64UrlEncode(new Uint8Array(digest))
}

function base64UrlEncode(input: Uint8Array) {
  let binary = ''
  input.forEach((value) => {
    binary += String.fromCharCode(value)
  })
  return window
    .btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/g, '')
}
