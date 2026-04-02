const authMode = import.meta.env.VITE_AODS_AUTH_MODE ?? 'header'
const issuerUrl = import.meta.env.VITE_AODS_OIDC_ISSUER_URL ?? ''
const clientId = import.meta.env.VITE_AODS_OIDC_CLIENT_ID ?? ''
const scope = import.meta.env.VITE_AODS_OIDC_SCOPE ?? 'openid profile email groups'
const redirectUriEnv = import.meta.env.VITE_AODS_OIDC_REDIRECT_URI ?? ''

const storageKeyPrefix = 'aods.oidc'
const accessTokenStorageKey = `${storageKeyPrefix}.access_token`
const refreshTokenStorageKey = `${storageKeyPrefix}.refresh_token`
const expiresAtStorageKey = `${storageKeyPrefix}.expires_at`
const stateStorageKey = `${storageKeyPrefix}.state`
const verifierStorageKey = `${storageKeyPrefix}.code_verifier`

type OIDCDiscoveryDocument = {
  authorization_endpoint: string
  token_endpoint: string
}

type TokenResponse = {
  access_token: string
  refresh_token?: string
  expires_in?: number
  refresh_expires_in?: number
}

let discoveryPromise: Promise<OIDCDiscoveryDocument> | null = null
let accessTokenPromise: Promise<string | undefined> | null = null

export function isOIDCAuthEnabled() {
  return authMode === 'oidc'
}

export async function ensureOIDCAccessToken() {
  if (!isOIDCAuthEnabled()) {
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
  window.sessionStorage.removeItem(refreshTokenStorageKey)
  window.sessionStorage.removeItem(expiresAtStorageKey)
  window.sessionStorage.removeItem(stateStorageKey)
  window.sessionStorage.removeItem(verifierStorageKey)
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
  const response = await fetch(discovery.token_endpoint, {
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
  const response = await fetch(discovery.token_endpoint, {
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
    discoveryPromise = fetch(discoveryURL).then(async (response) => {
      if (!response.ok) {
        throw new Error(`OIDC discovery failed with ${response.status}.`)
      }
      return (await response.json()) as OIDCDiscoveryDocument
    })
  }

  return discoveryPromise
}

function persistTokens(payload: TokenResponse) {
  if (!payload.access_token) {
    throw new Error('OIDC token response did not include access_token.')
  }

  window.sessionStorage.setItem(accessTokenStorageKey, payload.access_token)
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
