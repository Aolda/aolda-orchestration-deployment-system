import { describe, expect, it } from 'vitest'
import { render, screen } from '../../testing/test-utils'
import { ClustersPage } from './ClustersPage'

describe('ClustersPage', () => {
  it('[US-ADMIN-RES-001] platform admin에게 전체 리소스 효율 카드와 서비스 상세를 보여준다', () => {
    render(
      <ClustersPage
        clusters={[
          { id: 'default', name: 'Default Cluster', description: '기본 클러스터', default: true },
        ]}
        showAdminOverview
        adminOverview={{
          generatedAt: '2026-04-18T02:00:00Z',
          runtimeConnected: true,
          projectCount: 2,
          serviceCount: 1,
          capacity: {
            allocatableCpuCores: 16,
            requestedCpuCores: 6,
            usedCpuCores: 4.5,
            availableCpuCores: 10,
            allocatableMemoryMiB: 32768,
            requestedMemoryMiB: 8192,
            usedMemoryMiB: 4096,
            availableMemoryMiB: 24576,
            requestCpuUtilization: 37.5,
            usageCpuUtilization: 28.1,
            requestMemoryUtilization: 25,
            usageMemoryUtilization: 12.5,
          },
          counts: {
            balanced: 1,
            underutilized: 0,
            overutilized: 0,
            noMetrics: 0,
            unknown: 0,
          },
          services: [
            {
              applicationId: 'shared__portal',
              projectId: 'shared',
              projectName: '공용 프로젝트',
              clusterId: 'default',
              clusterName: 'Default Cluster',
              namespace: 'shared',
              name: 'portal',
              podCount: 2,
              readyPodCount: 2,
              status: 'Balanced',
              summary: '요청 대비 사용량이 현재 수준에서는 안정적입니다.',
              cpuUsageCores: 0.31,
              cpuRequestCores: 0.5,
              cpuLimitCores: 1,
              cpuRequestUtilization: 62,
              memoryUsageMiB: 320,
              memoryRequestMiB: 512,
              memoryLimitMiB: 1024,
              memoryRequestUtilization: 62.5,
            },
          ],
        }}
      />,
    )

    expect(screen.getByText('전체 리소스 효율')).toBeInTheDocument()
    expect(screen.getByText('서비스별 효율 상세')).toBeInTheDocument()
    expect(screen.getByText('공용 프로젝트')).toBeInTheDocument()
    expect(screen.getByText('portal')).toBeInTheDocument()
    expect(screen.getAllByText('안정').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Default Cluster').length).toBeGreaterThan(0)
  })
})
