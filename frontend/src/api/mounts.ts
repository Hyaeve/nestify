import { deleteJSON, getJSON, postJSON, putJSON } from './http'
import type { BrowseDirectoriesPayload } from './paths'

export { MOUNT_PATH_SCHEME, isMountPath } from './paths'

/** 挂载类型：通用 WebDAV 或 OpenList（后者支持原生递归列举生成 Strm）。 */
export type MountProvider = 'webdav' | 'openlist'

/** 认证方式：用户名 + 密码（Basic）或 OpenList 永久令牌（Bearer）。 */
export type MountAuthType = 'password' | 'token'

/** OpenList 默认监听端口，选定 OpenList 类型时自动填入。 */
export const OPENLIST_DEFAULT_PORT = 5244

export interface WebdavMount {
  id: number
  name: string
  provider: MountProvider
  auth_type: MountAuthType
  scheme: 'http' | 'https'
  host: string
  port: number
  username: string
  has_password: boolean
  /** 仅单条挂载（编辑）时返回，列表接口始终为空。 */
  password?: string
  has_token: boolean
  /** 与 password 同策略：仅编辑单条挂载时返回。 */
  token?: string
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
  provider: MountProvider
  auth_type: MountAuthType
  scheme: 'http' | 'https'
  host: string
  port: number
  username: string
  password: string
  token: string
  base_path: string
  enabled: boolean
  sort_order?: number
}

export interface MountListPayload {
  items: WebdavMount[]
}

/** 某个挂载被谁引用（删除前提示用）。路径里存的是 webdav://<id>，编号不会重排。 */
export interface MountUsage {
  mount_id: number
  rule_names: string[]
  backup_names: string[]
}

export function fetchMounts() {
  return getJSON<MountListPayload>('/api/v1/mounts')
}

/** 拉取引用该挂载的规则与备份任务名称（只做提示，不阻止删除）。 */
export function fetchMountUsage(id: number) {
  return getJSON<MountUsage>(`/api/v1/mounts/${id}/usage`)
}

/** 拉取单个挂载详情（含密码回显，仅编辑时使用）。 */
export function fetchMount(id: number) {
  return getJSON<WebdavMount>(`/api/v1/mounts/${id}`)
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
