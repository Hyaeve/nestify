export interface ApiResponse<T> {
  success: boolean
  code: string
  message: string
  data?: T
}

export async function getJSON<T>(url: string): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  const payload = (await response.json()) as ApiResponse<T>

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

	if (!response.ok) {
		throw new Error(payload.message || `Request failed with status ${response.status}`)
	}

	return payload
}

