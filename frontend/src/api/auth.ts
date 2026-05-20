import type { ApiResponse } from './http'

export interface SessionUser {
  id: number
  username: string
}

export async function login(username: string, password: string) {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  })

  const payload = (await response.json()) as ApiResponse<SessionUser>
  if (!response.ok) {
    throw new Error(payload.message || '登录失败')
  }

  return payload
}

export async function fetchCurrentSession() {
  const response = await fetch('/api/v1/auth/session', {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  const payload = (await response.json()) as ApiResponse<SessionUser>
  if (!response.ok) {
    throw new Error(payload.message || '获取会话失败')
  }

  return payload
}

export async function logout() {
  const response = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  const payload = (await response.json()) as ApiResponse<null>
  if (!response.ok) {
    throw new Error(payload.message || '退出失败')
  }

  return payload
}
