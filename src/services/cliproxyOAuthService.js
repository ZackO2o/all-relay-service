// cliproxyOAuthService — OAuth PKCE service via cliproxy-tls Go proxy.
// When CLAUDE_TLS_PROXY is set, all OAuth operations route through the Go proxy
// which uses uTLS transport (Chrome JA3 fingerprint) + device profile.
// This replaces the deprecated Cookie OAuth methods.
const axios = require('axios')
const config = require('../../config/config')
const logger = require('../utils/logger')

// Get the Go proxy base URL from env
function getBaseUrl() {
  return process.env.CLAUDE_TLS_PROXY || ''
}

// Generate an OAuth authorization URL via Go proxy
async function generateAuthUrl() {
  const baseUrl = getBaseUrl()
  if (!baseUrl) {
    throw new Error('CLAUDE_TLS_PROXY not configured')
  }

  const resp = await axios.get(`${baseUrl}/oauth/authorize`, {
    timeout: 15000,
    httpsAgent: new (require('https').Agent)({ rejectUnauthorized: false })
  })

  return resp.data // { auth_url, state, code_verifier, code_challenge, redirect_uri }
}

// Exchange authorization code for tokens via Go proxy
async function exchangeCode(code, codeVerifier) {
  const baseUrl = getBaseUrl()
  if (!baseUrl) {
    throw new Error('CLAUDE_TLS_PROXY not configured')
  }

  const resp = await axios.post(`${baseUrl}/oauth/callback`,
    { code, code_verifier: codeVerifier },
    {
      timeout: 30000,
      httpsAgent: new (require('https').Agent)({ rejectUnauthorized: false }),
      headers: { 'Content-Type': 'application/json' }
    }
  )

  return resp.data // { access_token, refresh_token, email, expire, last_refresh }
}

// Refresh tokens via Go proxy
async function refreshTokens(refreshToken) {
  const baseUrl = getBaseUrl()
  if (!baseUrl) {
    throw new Error('CLAUDE_TLS_PROXY not configured')
  }

  const resp = await axios.post(`${baseUrl}/oauth/refresh`,
    { refresh_token: refreshToken },
    {
      timeout: 30000,
      httpsAgent: new (require('https').Agent)({ rejectUnauthorized: false }),
      headers: { 'Content-Type': 'application/json' }
    }
  )

  return resp.data // { access_token, refresh_token, email, expire, last_refresh }
}

// Token exchange (compatibility with grant_type)
async function tokenExchange(grantType, params) {
  const baseUrl = getBaseUrl()
  if (!baseUrl) {
    throw new Error('CLAUDE_TLS_PROXY not configured')
  }

  const resp = await axios.post(`${baseUrl}/oauth/token`,
    { grant_type: grantType, ...params },
    {
      timeout: 30000,
      httpsAgent: new (require('https').Agent)({ rejectUnauthorized: false }),
      headers: { 'Content-Type': 'application/json' }
    }
  )

  return resp.data
}

// Check if cliproxy-tls is configured and accessible
async function healthCheck() {
  const baseUrl = getBaseUrl()
  if (!baseUrl) return false
  try {
    const resp = await axios.get(`${baseUrl}/health`, {
      timeout: 5000,
      httpsAgent: new (require('https').Agent)({ rejectUnauthorized: false })
    })
    return resp.data && resp.data.status === 'ok'
  } catch {
    return false
  }
}

module.exports = {
  generateAuthUrl,
  exchangeCode,
  refreshTokens,
  tokenExchange,
  healthCheck,
  getBaseUrl
}
