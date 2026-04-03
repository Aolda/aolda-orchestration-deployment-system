import { Badge, Table, Text } from '@mantine/core'
import type { ClusterSummary } from '../../types/api'
import { PageHeader } from '../../components/ui/PageHeader'
import { SurfaceCard } from '../../components/ui/SurfaceCard'

type ClustersPageProps = {
  clusters: ClusterSummary[]
}

export function ClustersPage({ clusters }: ClustersPageProps) {
  return (
    <>
      <PageHeader
        eyebrow="클러스터 카탈로그"
        title="클러스터"
        description="플랫폼이 인지하고 있는 배포 대상 클러스터 카탈로그입니다."
      />

      <SurfaceCard>
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
          <Text c="lagoon.4">아직 클러스터 카탈로그가 비어 있습니다.</Text>
        )}
      </SurfaceCard>
    </>
  )
}
