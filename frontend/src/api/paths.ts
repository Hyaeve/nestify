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
  /** 该条目是 WebDAV 虚拟挂载文件夹，而非物理目录。 */
  is_mount?: boolean
  /** 虚拟挂载的 http 根地址，仅用于展示。 */
  mount_host?: string
}

/** WebDAV 挂载虚拟路径前缀，例如 webdav://12/移动云盘/电视剧。 */
export const MOUNT_PATH_SCHEME = 'webdav://'

export function isMountPath(path?: string): boolean {
  return Boolean(path && path.startsWith(MOUNT_PATH_SCHEME))
}

export function mountIdFromPath(path: string): number | null {
  if (!isMountPath(path)) {
    return null
  }
  const rest = path.slice(MOUNT_PATH_SCHEME.length)
  const id = Number.parseInt(rest.split('/')[0] || '', 10)
  return Number.isFinite(id) && id > 0 ? id : null
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
	nest_source_folder?: boolean
	remove_subfolders?: boolean
}

export interface RenameItemPayload {
	path: string
	new_name: string
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

/** 统一入口：物理目录走本地浏览接口，WebDAV 挂载目录走挂载浏览接口。 */
export function browseAnyDirectory(path?: string) {
  if (isMountPath(path)) {
    return getJSON<BrowseDirectoriesPayload>(`/api/v1/mounts/browse?path=${encodeURIComponent(path || '')}`)
  }
  return browseDirectories(path)
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

export function renameItem(path: string, newName: string) {
	return postJSON<FileMutationResult>('/api/v1/files/rename', {
		path,
		new_name: newName,
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

export function packItemsAsCBZ(paths: string[], outputDir?: string, archiveName?: string, nestSourceFolder = true) {
  return postJSON<FileMutationResult>('/api/v1/files/pack-cbz', {
    paths,
    output_dir: outputDir || '',
    archive_name: archiveName || '',
    nest_source_folder: nestSourceFolder,
  })
}

export function packFoldersAsCBZ(paths: string[], removeSources = false) {
  return postJSON<FileMutationResult>('/api/v1/files/pack-folders-cbz', {
    paths,
    remove_sources: removeSources,
  })
}

export function extractArchives(paths: string[], outputDir?: string) {
  return postJSON<FileMutationResult>('/api/v1/files/extract', {
    paths,
    output_dir: outputDir || '',
  })
}

export function collectItems(paths: string[], removeSubfolders = false) {
	return postJSON<FileMutationResult>('/api/v1/files/collect', {
		paths,
		remove_subfolders: removeSubfolders,
	})
}

export async function uploadFiles(destinationPath: string, files: File[], relativePaths: string[] = []): Promise<ApiResponse<FileMutationResult>> {
  const formData = new FormData()
  formData.append('destination_path', destinationPath)
  files.forEach((file, index) => {
    formData.append('files', file)
    formData.append('relative_paths', relativePaths[index] || '')
  })

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
