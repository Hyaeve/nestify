export interface ApiResponse<T> {
  success: boolean
  code: string
  message: string
  data?: T
}

/** 后端「会话失效」的固定错误码：容器重启 / 会话过期后，业务接口都会带上它。 */
const UNAUTHORIZED_CODE = 'UNAUTHORIZED'

/**
 * 会话失效时的接管回调（由 main.ts 注册）：清空登录态并回登录窗口。
 * 返回 true 表示已接管本次请求。
 */
let sessionExpiredHandler: (() => boolean) | null = null

export function setSessionExpiredHandler(handler: () => boolean) {
  sessionExpiredHandler = handler
}

/**
 * 会话失效（容器重启 / 会话过期）时统一回登录窗口，不再出现「未登录」字样。
 *
 * 以前 401 被当成普通错误抛给页面：各页面弹出「未登录」提示，仪表盘还会把它
 * 渲染进顶部的错误提示条。现在命中后直接切到登录页，并要求调用方停在 pending
 * （见 haltRequest），于是页面既不弹提示、也不会渲染错误态。
 *
 * 判定同时看 status 与 code —— 登录密码错误同样是 401，但 code 是
 * INVALID_CREDENTIALS，必须放行给登录页正常提示。
 */
export function handleSessionExpired(response: Response, payload: ApiResponse<unknown>): boolean {
  if (response.status !== 401 || !payload || payload.code !== UNAUTHORIZED_CODE) {
    return false
  }

  return sessionExpiredHandler ? sessionExpiredHandler() : false
}

/** 让本次请求停在 pending：调用方既拿不到结果，也不会进入 catch 分支。 */
export function haltRequest<T>(): Promise<T> {
  return new Promise<T>(() => {})
}

export async function getJSON<T>(url: string): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  const payload = (await response.json()) as ApiResponse<T>

  if (handleSessionExpired(response, payload)) {
    return haltRequest<ApiResponse<T>>()
  }

  if (!response.ok) {
    throw new Error(payload.message || `Request failed with status ${response.status}`)
  }

  return payload
}

export async function postJSON<T>(url: string, body: unknown): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify(body),
  })

  const payload = (await response.json()) as ApiResponse<T>

  if (handleSessionExpired(response, payload)) {
    return haltRequest<ApiResponse<T>>()
  }

  if (!response.ok) {
    throw new Error(payload.message || `Request failed with status ${response.status}`)
  }

	return payload
}

export async function putJSON<T>(url: string, body: unknown): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify(body),
  })

  const payload = (await response.json()) as ApiResponse<T>

  if (handleSessionExpired(response, payload)) {
    return haltRequest<ApiResponse<T>>()
  }

  if (!response.ok) {
    throw new Error(payload.message || `Request failed with status ${response.status}`)
  }

  return payload
}

export async function deleteJSON<T>(url: string): Promise<ApiResponse<T>> {
	const response = await fetch(url, {
		method: 'DELETE',
		headers: {
			Accept: 'application/json',
		},
		credentials: 'include',
	})

	const payload = (await response.json()) as ApiResponse<T>

	if (handleSessionExpired(response, payload)) {
		return haltRequest<ApiResponse<T>>()
	}

	if (!response.ok) {
		throw new Error(payload.message || `Request failed with status ${response.status}`)
	}

	return payload
}

