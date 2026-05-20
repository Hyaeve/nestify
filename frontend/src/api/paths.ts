import { getJSON } from './http'

export interface BrowseRoot {
  name: string
  path: string
}

export interface BrowseRootsPayload {
  items: BrowseRoot[]
}

export interface DirectoryEntry {
  name: string
  path: string
  has_children: boolean
}

export interface BrowseDirectoriesPayload {
  current_path: string
  parent_path?: string
  entries: DirectoryEntry[]
}

export interface ValidatePathPayload {
  path: string
  allowed: boolean
  exists: boolean
  is_dir: boolean
  readable: boolean
  writable: boolean
}

export function fetchBrowseRoots() {
  return getJSON<BrowseRootsPayload>('/api/v1/paths/roots')
}

export function browseDirectories(path?: string) {
  const query = path ? `?path=${encodeURIComponent(path)}` : ''
  return getJSON<BrowseDirectoriesPayload>(`/api/v1/paths/browse${query}`)
}

export function validateDirectory(path: string) {
  return getJSON<ValidatePathPayload>(`/api/v1/paths/validate?path=${encodeURIComponent(path)}`)
}
