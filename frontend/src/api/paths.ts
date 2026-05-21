import { getJSON, postJSON, type ApiResponse } from './http'

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
  is_dir: boolean
  size: number
  modified_at: string
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

export interface CreateFolderPayload {
  parent_path: string
  name: string
}

export interface CreateFolderResult {
  path: string
}

export interface FileMutationPayload {
  paths: string[]
  destination_path?: string
  output_dir?: string
  archive_name?: string
}

export interface FileMutationResult {
  items?: string[]
  total: number
  output_path?: string
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

export function createFolder(parentPath: string, name: string) {
  return postJSON<CreateFolderResult>('/api/v1/files/create-folder', {
    parent_path: parentPath,
    name,
  })
}

export function copyItems(paths: string[], destinationPath: string) {
  return postJSON<FileMutationResult>('/api/v1/files/copy', {
    paths,
    destination_path: destinationPath,
  })
}

export function moveItems(paths: string[], destinationPath: string) {
  return postJSON<FileMutationResult>('/api/v1/files/move', {
    paths,
    destination_path: destinationPath,
  })
}

export function deleteItems(paths: string[]) {
  return postJSON<FileMutationResult>('/api/v1/files/delete', {
    paths,
  })
}

export function packItemsAsCBZ(paths: string[], outputDir?: string, archiveName?: string) {
  return postJSON<FileMutationResult>('/api/v1/files/pack-cbz', {
    paths,
    output_dir: outputDir || '',
    archive_name: archiveName || '',
  })
}

export async function uploadFiles(destinationPath: string, files: File[]): Promise<ApiResponse<FileMutationResult>> {
  const formData = new FormData()
  formData.append('destination_path', destinationPath)
  files.forEach((file) => formData.append('files', file))

  const response = await fetch('/api/v1/files/upload', {
    method: 'POST',
    credentials: 'include',
    body: formData,
  })

  const payload = (await response.json()) as ApiResponse<FileMutationResult>
  if (!response.ok) {
    throw new Error(payload.message || `Request failed with status ${response.status}`)
  }

  return payload
}
