import { Badge, Group, List, SimpleGrid, Text } from '@mantine/core'
import type { CurrentUser, ProjectSummary } from '../../types/api'
import { PageHeader } from '../../components/ui/PageHeader'
import { SurfaceCard } from '../../components/ui/SurfaceCard'

type MePageProps = {
  user: CurrentUser | null
  projects: ProjectSummary[]
}

export function MePage({ user, projects }: MePageProps) {
  return (
    <>
      <PageHeader
        eyebrow="사용자 정보"
        title="내 정보"
        description="현재 로그인 사용자, 그룹, 접근 가능한 프로젝트를 보여줍니다."
      />

      <SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
        <SurfaceCard>
          <Text fw={700} c="lagoon.8" mb="sm">
            사용자
          </Text>
          <Text c="lagoon.9">{user?.displayName || user?.username || '이름 없음'}</Text>
          <Text c="lagoon.4">{user?.id || '-'}</Text>
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
              <Text c="lagoon.4">그룹 없음</Text>
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
            <Text c="lagoon.4">프로젝트 없음</Text>
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
