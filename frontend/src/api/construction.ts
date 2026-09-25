import { apiDelete, apiGet, apiPost, apiPut } from '@/utils/request'
import type {
  AcceptConstructionRequest,
  ConstructionNode,
  CreateConstructionRequest,
  MaterialUsage,
  PageResult,
  RegisterMaterialUsageRequest,
} from '@/types'
import { API_PATHS } from '@/constants/apiPaths'

export function listConstructions(params?: Record<string, unknown>): Promise<PageResult<ConstructionNode> | ConstructionNode[]> {
  return apiGet<PageResult<ConstructionNode> | ConstructionNode[]>(API_PATHS.constructions, params)
}

export function getConstruction(id: number): Promise<ConstructionNode> {
  return apiGet<ConstructionNode>(`${API_PATHS.constructions}/${id}`)
}

export function createConstruction(body: CreateConstructionRequest): Promise<ConstructionNode> {
  return apiPost<ConstructionNode>(API_PATHS.constructions, body)
}

export function updateConstruction(id: number, body: Partial<CreateConstructionRequest>): Promise<ConstructionNode> {
  return apiPut<ConstructionNode>(`${API_PATHS.constructions}/${id}`, body)
}

export function deleteConstruction(id: number): Promise<void> {
  return apiDelete<void>(`${API_PATHS.constructions}/${id}`)
}

export function updateConstructionStatus(id: number, status: string): Promise<ConstructionNode> {
  return apiPut<ConstructionNode>(`${API_PATHS.constructions}/${id}/status`, { status })
}

export function acceptConstruction(id: number, body: AcceptConstructionRequest): Promise<ConstructionNode> {
  return apiPut<ConstructionNode>(`${API_PATHS.constructions}/${id}/accept`, body)
}

// 节点完工前登记本次用料。client_key 用于同一记录重复提交时保留第一次结果。
export function registerNodeUsage(nodeId: number, body: RegisterMaterialUsageRequest): Promise<MaterialUsage> {
  return apiPost<MaterialUsage>(`${API_PATHS.constructions}/${nodeId}/usages`, body)
}

export function listNodeUsages(nodeId: number): Promise<MaterialUsage[]> {
  return apiGet<MaterialUsage[]>(`${API_PATHS.constructions}/${nodeId}/usages`)
}

export function deleteMaterialUsage(id: number): Promise<void> {
  return apiDelete<void>(`${API_PATHS.materialUsages}/${id}`)
}
