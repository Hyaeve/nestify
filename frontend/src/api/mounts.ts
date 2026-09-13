import { deleteJSON, getJSON, postJSON, putJSON } from './http'
import type { BrowseDirectoriesPayload } from './paths'

export { MOUNT_PATH_SCHEME, isMountPath } from './paths'

export interface WebdavMount {
  id: number
  name: string
  scheme: 'http' | 'https'
  host: string
  port: number
  username: string
  has_password: boolean
  base_path: string
  enabled: boolean
  sort_order: number
  base_url: string
  virtual_path: string
  created_at: string
  updated_at: string
}

export interface MountInput {
  name: string
  scheme: 'http' | 'https'
  host: string
  port: number
  username: string
  password: string
  base_path: string
  enabled: boolean
  sort_order?: number
}

export interface MountListPayload {
  items: WebdavMount[]
}

export function fetchMounts() {
  return getJSON<MountListPayload>('/api/v1/mounts')
}

export function createMount(payload: MountInput) {
  return postJSON<WebdavMount>('/api/v1/mounts', payload)
}

export function updateMount(id: number, payload: MountInput) {
  return putJSON<WebdavMount>(`/api/v1/mounts/${id}`, payload)
}

export function deleteMount(id: number) {
  return deleteJSON<Record<string, never>>(`/api/v1/mounts/${id}`)
}

export function browseMountDirectory(path: string) {
  return getJSON<BrowseDirectoriesPayload>(`/api/v1/mounts/browse?path=${encodeURIComponent(path)}`)
}

export function testMountConnection(payload: MountInput) {
  return postJSON<{ count: number }>('/api/v1/mounts/test', payload)
}
