import { useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
  Upload,
} from 'antd'
import { CheckOutlined, FormOutlined, PlayCircleOutlined, UploadOutlined } from '@ant-design/icons'
import StatusBadge from '@/components/common/StatusBadge'
import Timeline from '@/components/common/Timeline'
import StepIndicator from '@/components/common/StepIndicator'
import { useConstructionStore } from '@/stores/constructionStore'
import { useProjectStore } from '@/stores/projectStore'
import { useMaterialStore } from '@/stores/materialStore'
import { useAuthStore } from '@/stores/authStore'
import { acceptConstruction, createConstruction, updateConstructionStatus } from '@/api/construction'
import { registerMaterialUsage } from '@/api/materialUsage'
import { uploadFile } from '@/utils/upload'
import { extractErrorMessage } from '@/utils/request'
import { ConstructionStatus, MaterialUsageStatus, Role, ConstructionName } from '@/types/enums'
import type { ConstructionNode, MaterialItem, MaterialUsage } from '@/types'

const usageStatusLabel: Record<string, { text: string; color: string }> = {
  [MaterialUsageStatus.Pending]: { text: '待计入', color: 'default' },
  [MaterialUsageStatus.Counted]: { text: '已计入', color: 'green' },
  [MaterialUsageStatus.Excluded]: { text: '待确认', color: 'orange' },
}

export default function ConstructionProgress() {
  const { nodes, fetchNodes } = useConstructionStore()
  const { projects, fetchProjects } = useProjectStore()
  const { materials, fetchMaterials } = useMaterialStore()
  const user = useAuthStore((state) => state.user)
  const [projectId, setProjectId] = useState<number>()
  const [acceptTarget, setAcceptTarget] = useState<ConstructionNode | null>(null)
  const [usageTarget, setUsageTarget] = useState<ConstructionNode | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [photoUrls, setPhotoUrls] = useState<string[]>([])
  const [form] = Form.useForm()
  const [acceptForm] = Form.useForm()
  const [usageForm] = Form.useForm()

  useEffect(() => {
    fetchProjects()
  }, [fetchProjects])

  useEffect(() => {
    fetchNodes(projectId)
    fetchMaterials(projectId)
  }, [fetchNodes, fetchMaterials, projectId])

  const filtered = useMemo(() => {
    if (!projectId) return nodes
    return nodes.filter((n) => n.project_id === projectId)
  }, [nodes, projectId])

  const projectMaterials = useMemo<MaterialItem[]>(() => {
    if (!projectId) return materials
    return materials.filter((m) => m.project_id === projectId)
  }, [materials, projectId])

  // 可登记余量 = 采购量 - 已计入 - 全项目已登记待计入（待确认不占用）。
  const availableQuantity = (material: MaterialItem) => {
    const pending = nodes
      .filter((n) => n.project_id === material.project_id && n.id !== usageTarget?.id)
      .flatMap((n) => n.usages || [])
      .filter((u) => u.material_id === material.id && u.status === MaterialUsageStatus.Pending)
      .reduce((sum, u) => sum + u.quantity, 0)
    return Math.max(0, material.quantity - material.installed_quantity - pending)
  }

  const canOperate = user?.role === Role.Admin || user?.role === Role.Contractor || user?.role === Role.ProjectManager

  const startNode = async (node: ConstructionNode) => {
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.InProgress)
      message.success('节点已开工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const completeNode = async (node: ConstructionNode) => {
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.Completed)
      message.success('节点已完工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onAccept = async () => {
    const values = await acceptForm.validateFields()
    if (!acceptTarget) return
    try {
      await acceptConstruction(acceptTarget.id, { ...values, photos: photoUrls })
      message.success('验收完成')
      setAcceptTarget(null)
      setPhotoUrls([])
      acceptForm.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onCreate = async () => {
    const values = await form.validateFields()
    try {
      await createConstruction({ ...values, project_id: projectId! })
      message.success('创建成功')
      setCreateOpen(false)
      form.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onRegisterUsage = async () => {
    const values = await usageForm.validateFields()
    if (!usageTarget) return
    try {
      await registerMaterialUsage({ node_id: usageTarget.id, ...values })
      message.success('用料已登记')
      setUsageTarget(null)
      usageForm.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const renderUsages = (usages: MaterialUsage[]) => {
    if (!usages || usages.length === 0) {
      return <div style={{ color: '#8c8c8c', padding: '8px 0' }}>尚未登记用料</div>
    }
    return (
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={usages}
        columns={[
          { title: '材料', dataIndex: 'material_name' },
          { title: '数量', render: (_, record) => `${record.quantity} ${record.unit}` },
          {
            title: '状态',
            dataIndex: 'status',
            render: (v: string) => {
              const meta = usageStatusLabel[v] || { text: v, color: 'default' }
              return <Tag color={meta.color}>{meta.text}</Tag>
            },
          },
          { title: '备注', dataIndex: 'remark' },
        ]}
      />
    )
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={3} style={{ margin: 0 }}>施工进度</Typography.Title>
        <Select
          style={{ width: 240 }}
          placeholder="选择项目"
          allowClear
          value={projectId}
          onChange={setProjectId}
          options={projects.map((p) => ({ label: p.name, value: p.id }))}
        />
        {user?.role === Role.Admin || user?.role === Role.ProjectManager ? (
          <Button type="primary" disabled={!projectId} onClick={() => setCreateOpen(true)}>新增节点</Button>
        ) : null}
      </Space>

      <StepIndicator
        current={filtered.filter((n) => n.status === ConstructionStatus.Completed).length}
        items={filtered.map((n) => n.name)}
      />

      <Card style={{ marginTop: 16 }}>
        <Timeline
          items={filtered.map((node) => ({
            title: node.name,
            status: node.status,
            time: `${node.planned_start_date || '-'} ~ ${node.planned_end_date || '-'}`,
            description: `验收状态：${node.acceptance_status}${node.acceptance_note ? ' · ' + node.acceptance_note : ''}`,
          }))}
        />
      </Card>

      <Card title="节点操作（展开查看用料）" style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          dataSource={filtered}
          pagination={false}
          expandable={{
            expandedRowRender: (record) => renderUsages(record.usages),
            rowExpandable: () => true,
          }}
          columns={[
            { title: '节点名称', dataIndex: 'name' },
            { title: '状态', dataIndex: 'status', render: (v) => <StatusBadge status={v} /> },
            { title: '验收状态', dataIndex: 'acceptance_status', render: (v) => <StatusBadge status={v} /> },
            {
              title: '用料',
              render: (_, record) => {
                const counted = (record.usages || []).filter((u) => u.status === MaterialUsageStatus.Counted).length
                const pending = (record.usages || []).filter((u) => u.status === MaterialUsageStatus.Pending).length
                return (
                  <Space size={4}>
                    <Tag color="green">已计入 {counted}</Tag>
                    <Tag>待计入 {pending}</Tag>
                  </Space>
                )
              },
            },
            { title: '计划开始', dataIndex: 'planned_start_date' },
            { title: '计划结束', dataIndex: 'planned_end_date' },
            {
              title: '操作',
              render: (_, record) =>
                canOperate ? (
                  <Space wrap>
                    {record.status === ConstructionStatus.Pending ? (
                      <Button size="small" icon={<PlayCircleOutlined />} onClick={() => startNode(record)}>开工</Button>
                    ) : null}
                    {record.status === ConstructionStatus.InProgress ? (
                      <Space size={4}>
                        <Button size="small" icon={<FormOutlined />} onClick={() => setUsageTarget(record)}>
                          登记用料
                        </Button>
                        <Button size="small" type="primary" onClick={() => completeNode(record)}>完工</Button>
                      </Space>
                    ) : null}
                    {record.status === ConstructionStatus.Completed ? (
                      <Button size="small" icon={<CheckOutlined />} onClick={() => setAcceptTarget(record)}>
                        {record.acceptance_status === 'Pending' ? '验收' : '重新验收'}
                      </Button>
                    ) : null}
                  </Space>
                ) : null,
            },
          ]}
        />
      </Card>

      <Modal title="新增施工节点" open={createOpen} onOk={onCreate} onCancel={() => setCreateOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="节点名称" rules={[{ required: true }]}>
            <Select options={ConstructionName.map((n) => ({ label: n, value: n }))} />
          </Form.Item>
          <Form.Item name="planned_start_date" label="计划开始日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
          <Form.Item name="planned_end_date" label="计划结束日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`登记用料 · ${usageTarget?.name || ''}`}
        open={!!usageTarget}
        onOk={onRegisterUsage}
        onCancel={() => setUsageTarget(null)}
        okText="登记"
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          节点完工前登记本次用料；验收通过后计入材料已安装量，同一材料重复提交只保留第一次结果。
        </Typography.Paragraph>
        <Form form={usageForm} layout="vertical">
          <Form.Item name="material_id" label="材料" rules={[{ required: true, message: '请选择材料' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择本项目材料"
              options={projectMaterials.map((m) => ({
                label: `${m.name}（可登记 ${availableQuantity(m)} ${m.unit}）`,
                value: m.id,
              }))}
            />
          </Form.Item>
          <Form.Item name="quantity" label="本次用料数量" rules={[{ required: true, message: '请输入数量' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input maxLength={255} placeholder="使用部位/说明" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="施工验收" open={!!acceptTarget} onOk={onAccept} onCancel={() => setAcceptTarget(null)}>
        <Form form={acceptForm} layout="vertical" initialValues={{ accepted: true }}>
          <Form.Item name="accepted" label="验收结论" rules={[{ required: true }]}>
            <Select options={[{ label: '验收通过', value: true }, { label: '验收不通过', value: false }]} />
          </Form.Item>
          <Form.Item name="note" label="验收说明"><Input.TextArea rows={3} /></Form.Item>
          <Form.Item label="验收照片">
            <Upload
              beforeUpload={async (file) => {
                try {
                  const url = await uploadFile(file)
                  setPhotoUrls((prev) => [...prev, url])
                } catch (error) {
                  message.error(extractErrorMessage(error))
                }
                return false
              }}
            >
              <Button icon={<UploadOutlined />}>上传照片</Button>
            </Upload>
            {photoUrls.map((url) => (
              <div key={url} style={{ marginTop: 8 }}>{url}</div>
            ))}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
