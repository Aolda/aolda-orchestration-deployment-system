import { Badge, Group, List, SimpleGrid, Text } from '@mantine/core'
import type { CurrentUser, ProjectSummary } from '../../types/api'
import { PageHeader } from '../../components/ui/PageHeader'
import { SurfaceCard } from '../../components/ui/SurfaceCard'
import { StatePanel } from '../../components/ui/StatePanel'

type MePageProps = {
  user: CurrentUser | null
  projects: ProjectSummary[]
  loading?: boolean
  errorMessage?: string | null
}

export function MePage({ user, projects, loading = false, errorMessage }: MePageProps) {
  return (
    <>
      <PageHeader
        eyebrow="사용자 정보"
        title="내 정보"
        description="현재 로그인 사용자, 그룹, 접근 가능한 프로젝트를 보여줍니다."
      />

      {loading ? (
        <StatePanel
          kind="loading"
          title="내 정보를 불러오는 중"
          description="현재 로그인 사용자와 접근 가능한 프로젝트를 조회하고 있습니다."
        />
      ) : errorMessage ? (
        <StatePanel
          kind="partial"
          title="일부 사용자 정보를 불러오지 못했습니다"
          description={errorMessage}
        />
      ) : null}

      <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
        <SurfaceCard>
          <Text fw={700} c="lagoon.8" mb="sm">
            사용자
          </Text>
          {user ? (
            <>
              <Text c="lagoon.9">{user.displayName || user.username || '이름 없음'}</Text>
              <Text c="lagoon.4">{user.id || '-'}</Text>
            </>
          ) : (
            <StatePanel
              kind="partial"
              withCard={false}
              title="사용자 메타데이터를 받지 못했습니다"
              description="세션은 열려 있지만 사용자 식별 정보가 비어 있습니다."
            />
          )}
        </SurfaceCard>

        <SurfaceCard>
          <Text fw={700} c="lagoon.8" mb="sm">
            그룹
          </Text>
          <Group gap="xs">
            {user?.groups?.length ? (
              user.groups.map((group) => (
                <Badge key={group} color="lagoon.6" variant="light" radius="sm">
                  {group}
                </Badge>
              ))
            ) : (
              <StatePanel
                kind="empty"
                withCard={false}
                title="연결된 그룹이 없습니다"
                description="추가 그룹 매핑이 없거나 현재 사용자에게 그룹 정보가 노출되지 않았습니다."
              />
            )}
          </Group>
        </SurfaceCard>

        <SurfaceCard>
          <Text fw={700} c="lagoon.8" mb="sm">
            접근 가능한 프로젝트
          </Text>
          {projects.length > 0 ? (
            <List spacing="xs" size="sm">
              {projects.map((project) => (
                <List.Item key={project.id}>
                  {project.name} ({formatRoleLabel(project.role)})
                </List.Item>
              ))}
            </List>
          ) : (
            <StatePanel
              kind="empty"
              withCard={false}
              title="접근 가능한 프로젝트가 없습니다"
              description="현재 계정에 연결된 프로젝트 권한이 없거나 아직 프로젝트가 구성되지 않았습니다."
            />
          )}
        </SurfaceCard>
      </SimpleGrid>
    </>
  )
}

function formatRoleLabel(role: ProjectSummary['role']) {
  switch (role) {
    case 'admin':
      return '관리자'
    case 'deployer':
      return '배포자'
    default:
      return '조회자'
  }
}
