import { apiGet, apiPost } from '@/utils/request'
import type { MaterialUsage, RegisterMaterialUsageRequest } from '@/types'
import { API_PATHS } from '@/constants/apiPaths'

export function listMaterialUsages(nodeId: number): Promise<MaterialUsage[]> {
  return apiGet<MaterialUsage[]>(`${API_PATHS.materialUsages}/node`, { node_id: nodeId })
}

// 登记节点用料；重复提交时后端保留第一次结果并返回该记录。
export function registerMaterialUsage(body: RegisterMaterialUsageRequest): Promise<MaterialUsage> {
  return apiPost<MaterialUsage>(API_PATHS.materialUsages, body)
}
