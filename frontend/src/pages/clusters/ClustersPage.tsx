import type { ReactNode } from 'react'
import {
  Badge,
  Group,
  Progress,
  ScrollArea,
  SimpleGrid,
  Stack,
  Table,
  Text,
} from '@mantine/core'
import type {
  ClusterSummary,
  FleetResourceOverviewResponse,
  ServiceEfficiencyStatus,
  ServiceResourceEfficiency,
} from '../../types/api'
import { PageHeader } from '../../components/ui/PageHeader'
import { SurfaceCard } from '../../components/ui/SurfaceCard'
import { StatePanel } from '../../components/ui/StatePanel'

type ClustersPageProps = {
  clusters: ClusterSummary[]
  loading?: boolean
  errorMessage?: string | null
  showAdminOverview?: boolean
  adminOverview?: FleetResourceOverviewResponse | null
  adminOverviewLoading?: boolean
  adminOverviewError?: string | null
  actions?: ReactNode
  creationPanel?: ReactNode
}

export function ClustersPage({
  clusters,
  loading = false,
  errorMessage,
  showAdminOverview = false,
  adminOverview,
  adminOverviewLoading = false,
  adminOverviewError,
  actions,
  creationPanel,
}: ClustersPageProps) {
  return (
    <>
      <PageHeader
        eyebrow="클러스터 카탈로그"
        title="클러스터"
        description="플랫폼이 인지하고 있는 배포 대상 클러스터 카탈로그와, platform admin 전용 전체 리소스 효율 현황입니다."
        actions={actions}
      />

      {creationPanel}

      {loading ? (
        <StatePanel
          kind="loading"
          title="클러스터 카탈로그를 불러오는 중"
          description="배포 대상 클러스터 메타데이터를 가져오고 있습니다."
        />
      ) : errorMessage ? (
        <StatePanel
          kind="partial"
          title="클러스터 카탈로그를 완전히 불러오지 못했습니다"
          description={errorMessage}
        />
      ) : null}

      {showAdminOverview ? (
        <AdminOverviewSection
          overview={adminOverview}
          loading={adminOverviewLoading}
          errorMessage={adminOverviewError}
        />
      ) : null}

      <SurfaceCard>
        <Stack gap="md">
          <div>
            <Text fw={800} c="lagoon.9">
              클러스터 카탈로그
            </Text>
            <Text size="sm" c="lagoon.4" mt={4}>
              프로젝트와 운영 환경이 참조하는 배포 대상 클러스터 목록입니다.
            </Text>
          </div>

          {clusters.length > 0 ? (
            <Table striped highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>이름</Table.Th>
                  <Table.Th>식별자</Table.Th>
                  <Table.Th>기본 여부</Table.Th>
                  <Table.Th>설명</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {clusters.map((cluster) => (
                  <Table.Tr key={cluster.id}>
                    <Table.Td>{cluster.name}</Table.Td>
                    <Table.Td>{cluster.id}</Table.Td>
                    <Table.Td>
                      <Badge color={cluster.default ? 'lagoon.6' : 'lagoon.4'} variant="light">
                        {cluster.default ? '기본' : '일반'}
                      </Badge>
                    </Table.Td>
                    <Table.Td>{cluster.description || '-'}</Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          ) : (
            <StatePanel
              kind="empty"
              withCard={false}
              title="표시할 클러스터가 없습니다"
              description="플랫폼이 인지하는 클러스터가 아직 등록되지 않았거나 현재 사용자에게 노출된 항목이 없습니다."
            />
          )}
        </Stack>
      </SurfaceCard>
    </>
  )
}

type AdminOverviewSectionProps = {
  overview?: FleetResourceOverviewResponse | null
  loading: boolean
  errorMessage?: string | null
}

function AdminOverviewSection({ overview, loading, errorMessage }: AdminOverviewSectionProps) {
  if (loading && !overview) {
    return (
      <StatePanel
        kind="loading"
        title="전체 리소스 효율을 계산하는 중"
        description="현재 연결된 Kubernetes runtime의 총량과 서비스별 효율을 집계하고 있습니다."
      />
    )
  }

  if (!overview && errorMessage) {
    return (
      <StatePanel
        kind="partial"
        title="전체 리소스 효율 정보를 불러오지 못했습니다"
        description={errorMessage}
      />
    )
  }

  if (!overview) {
    return null
  }

  return (
    <Stack gap="md">
      {errorMessage ? (
        <StatePanel
          kind="partial"
          title="최신 리소스 효율 화면을 완전히 갱신하지 못했습니다"
          description={errorMessage}
        />
      ) : null}

      <SurfaceCard>
        <Stack gap="lg">
          <Group justify="space-between" align="flex-start">
            <div>
              <Text fw={800} c="lagoon.9">
                전체 리소스 효율
              </Text>
              <Text size="sm" c="lagoon.4" mt={4}>
                현재 연결된 Kubernetes runtime 기준으로 전체 할당 가능 리소스, 요청량, 실사용량, 서비스 효율을 요약합니다.
              </Text>
            </div>
            <Badge color={overview.runtimeConnected ? 'green' : 'yellow'} variant="light" size="lg">
              {overview.runtimeConnected ? 'runtime 연결됨' : 'runtime 제한 모드'}
            </Badge>
          </Group>

          <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="md">
            <MetricTile
              label="관리 프로젝트"
              value={`${overview.projectCount}`}
              description="platform/projects.yaml 기준"
            />
            <MetricTile
              label="관리 서비스"
              value={`${overview.serviceCount}`}
              description="AODS가 추적 중인 애플리케이션"
            />
            <MetricTile
              label="남은 CPU"
              value={formatCores(overview.capacity.availableCpuCores)}
              description={`할당 가능 ${formatCores(overview.capacity.allocatableCpuCores)} / 요청 ${formatCores(overview.capacity.requestedCpuCores)}`}
            />
            <MetricTile
              label="남은 메모리"
              value={formatMemory(overview.capacity.availableMemoryMiB)}
              description={`할당 가능 ${formatMemory(overview.capacity.allocatableMemoryMiB)} / 요청 ${formatMemory(overview.capacity.requestedMemoryMiB)}`}
            />
          </SimpleGrid>

          {overview.message ? (
            <Text size="sm" c={overview.runtimeConnected ? 'lagoon.4' : 'orange.8'}>
              {overview.message}
            </Text>
          ) : null}

          <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
            <SurfaceCard>
              <Stack gap="sm">
                <Text fw={800} c="lagoon.9">
                  클러스터 점유율
                </Text>
                <UtilizationRow
                  label="CPU 요청률"
                  value={overview.capacity.requestCpuUtilization}
                  detail={`${formatCores(overview.capacity.requestedCpuCores)} / ${formatCores(overview.capacity.allocatableCpuCores)}`}
                />
                <UtilizationRow
                  label="CPU 실사용률"
                  value={overview.capacity.usageCpuUtilization}
                  detail={`${formatCores(overview.capacity.usedCpuCores)} / ${formatCores(overview.capacity.allocatableCpuCores)}`}
                />
                <UtilizationRow
                  label="메모리 요청률"
                  value={overview.capacity.requestMemoryUtilization}
                  detail={`${formatMemory(overview.capacity.requestedMemoryMiB)} / ${formatMemory(overview.capacity.allocatableMemoryMiB)}`}
                />
                <UtilizationRow
                  label="메모리 실사용률"
                  value={overview.capacity.usageMemoryUtilization}
                  detail={`${formatMemory(overview.capacity.usedMemoryMiB)} / ${formatMemory(overview.capacity.allocatableMemoryMiB)}`}
                />
              </Stack>
            </SurfaceCard>

            <SurfaceCard>
              <Stack gap="md">
                <div>
                  <Text fw={800} c="lagoon.9">
                    서비스 효율 분포
                  </Text>
                  <Text size="sm" c="lagoon.4" mt={4}>
                    요청치 대비 실사용률을 기준으로 과사용, 저사용, 안정 상태를 구분했습니다.
                  </Text>
                </div>
                <SimpleGrid cols={{ base: 2, md: 5 }} spacing="sm">
                  <MetricTile label="과사용" value={`${overview.counts.overutilized}`} tone="red" />
                  <MetricTile label="저사용" value={`${overview.counts.underutilized}`} tone="yellow" />
                  <MetricTile label="안정" value={`${overview.counts.balanced}`} tone="green" />
                  <MetricTile label="메트릭 없음" value={`${overview.counts.noMetrics}`} tone="gray" />
                  <MetricTile label="미확인" value={`${overview.counts.unknown}`} tone="blue" />
                </SimpleGrid>
              </Stack>
            </SurfaceCard>
          </SimpleGrid>
        </Stack>
      </SurfaceCard>

      <SurfaceCard>
        <Stack gap="md">
          <div>
            <Text fw={800} c="lagoon.9">
              서비스별 효율 상세
            </Text>
            <Text size="sm" c="lagoon.4" mt={4}>
              모든 프로젝트와 서비스에 대해 pod 준비 상태, 요청/제한/실사용 리소스, 효율 판정을 한 번에 볼 수 있습니다.
            </Text>
          </div>

          {overview.services.length > 0 ? (
            <ScrollArea offsetScrollbars>
              <Table striped highlightOnHover miw={1280}>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>프로젝트</Table.Th>
                    <Table.Th>서비스</Table.Th>
                    <Table.Th>클러스터</Table.Th>
                    <Table.Th>네임스페이스</Table.Th>
                    <Table.Th>Ready / Pod</Table.Th>
                    <Table.Th>CPU (사용 / 요청 / 제한)</Table.Th>
                    <Table.Th>CPU 요청 효율</Table.Th>
                    <Table.Th>메모리 (사용 / 요청 / 제한)</Table.Th>
                    <Table.Th>메모리 요청 효율</Table.Th>
                    <Table.Th>상태</Table.Th>
                    <Table.Th>요약</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {overview.services.map((service) => (
                    <Table.Tr key={service.applicationId}>
                      <Table.Td>{service.projectName}</Table.Td>
                      <Table.Td>
                        <Stack gap={2}>
                          <Text fw={700}>{service.name}</Text>
                          <Text size="xs" c="lagoon.4">
                            {service.applicationId}
                          </Text>
                        </Stack>
                      </Table.Td>
                      <Table.Td>{service.clusterName || service.clusterId || '-'}</Table.Td>
                      <Table.Td>{service.namespace}</Table.Td>
                      <Table.Td>{`${service.readyPodCount} / ${service.podCount}`}</Table.Td>
                      <Table.Td>{formatTripleCores(service)}</Table.Td>
                      <Table.Td>{formatPercent(service.cpuRequestUtilization)}</Table.Td>
                      <Table.Td>{formatTripleMemory(service)}</Table.Td>
                      <Table.Td>{formatPercent(service.memoryRequestUtilization)}</Table.Td>
                      <Table.Td>
                        <Badge color={statusColor(service.status)} variant="light">
                          {statusLabel(service.status)}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" maw={320}>
                          {service.summary}
                        </Text>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </ScrollArea>
          ) : (
            <StatePanel
              kind="empty"
              withCard={false}
              title="효율 계산 대상 서비스가 없습니다"
              description="현재 프로젝트 카탈로그에 등록된 애플리케이션이 없거나 아직 집계할 수 있는 서비스가 없습니다."
            />
          )}
        </Stack>
      </SurfaceCard>
    </Stack>
  )
}

type MetricTileProps = {
  label: string
  value: string
  description?: string
  tone?: 'green' | 'yellow' | 'red' | 'gray' | 'blue'
}

function MetricTile({ label, value, description, tone = 'blue' }: MetricTileProps) {
  const color = tileColor(tone)

  return (
    <div
      style={{
        border: '1px solid var(--portal-border)',
        borderRadius: 18,
        padding: '16px 18px',
        background: color.background,
      }}
    >
      <Text size="xs" fw={800} c="lagoon.4" tt="uppercase">
        {label}
      </Text>
      <Text fw={900} size="xl" c={color.value}>
        {value}
      </Text>
      {description ? (
        <Text size="sm" c="lagoon.4" mt={6}>
          {description}
        </Text>
      ) : null}
    </div>
  )
}

type UtilizationRowProps = {
  label: string
  value?: number
  detail: string
}

function UtilizationRow({ label, value, detail }: UtilizationRowProps) {
  return (
    <div>
      <Group justify="space-between" mb={6}>
        <Text size="sm" fw={700} c="lagoon.9">
          {label}
        </Text>
        <Text size="sm" c="lagoon.4">
          {formatPercent(value)}
        </Text>
      </Group>
      <Progress color={progressColor(value)} radius="xl" size="lg" value={clampPercent(value)} />
      <Text size="xs" c="lagoon.4" mt={6}>
        {detail}
      </Text>
    </div>
  )
}

function formatTripleCores(service: ServiceResourceEfficiency) {
  return `${formatCores(service.cpuUsageCores)} / ${formatCores(service.cpuRequestCores)} / ${formatCores(service.cpuLimitCores)}`
}

function formatTripleMemory(service: ServiceResourceEfficiency) {
  return `${formatMemory(service.memoryUsageMiB)} / ${formatMemory(service.memoryRequestMiB)} / ${formatMemory(service.memoryLimitMiB)}`
}

function formatCores(value?: number) {
  if (value == null) {
    return '-'
  }
  if (value < 1) {
    return `${Math.round(value * 1000)}m`
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} cores`
}

function formatMemory(value?: number) {
  if (value == null) {
    return '-'
  }
  if (value >= 1024) {
    const gib = value / 1024
    return `${gib.toFixed(gib >= 10 ? 1 : 2)} GiB`
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} MiB`
}

function formatPercent(value?: number) {
  if (value == null) {
    return '-'
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)}%`
}

function clampPercent(value?: number) {
  if (value == null || Number.isNaN(value)) {
    return 0
  }
  return Math.max(0, Math.min(100, value))
}

function progressColor(value?: number) {
  if (value == null) {
    return 'gray'
  }
  if (value > 85) {
    return 'red'
  }
  if (value > 60) {
    return 'yellow'
  }
  return 'lagoon.6'
}

function statusLabel(status: ServiceEfficiencyStatus) {
  switch (status) {
    case 'Overutilized':
      return '과사용'
    case 'Underutilized':
      return '저사용'
    case 'Balanced':
      return '안정'
    case 'NoMetrics':
      return '메트릭 없음'
    default:
      return '미확인'
  }
}

function statusColor(status: ServiceEfficiencyStatus) {
  switch (status) {
    case 'Overutilized':
      return 'red'
    case 'Underutilized':
      return 'yellow'
    case 'Balanced':
      return 'green'
    case 'NoMetrics':
      return 'gray'
    default:
      return 'blue'
  }
}

function tileColor(tone: MetricTileProps['tone']) {
  switch (tone) {
    case 'green':
      return { background: 'rgba(46, 160, 67, 0.08)', value: 'var(--mantine-color-green-8)' }
    case 'yellow':
      return { background: 'rgba(245, 159, 0, 0.08)', value: 'var(--mantine-color-yellow-8)' }
    case 'red':
      return { background: 'rgba(250, 82, 82, 0.08)', value: 'var(--mantine-color-red-8)' }
    case 'gray':
      return { background: 'rgba(134, 142, 150, 0.08)', value: 'var(--mantine-color-gray-7)' }
    default:
      return { background: 'rgba(46, 132, 255, 0.08)', value: 'var(--mantine-color-lagoon-8)' }
  }
}
