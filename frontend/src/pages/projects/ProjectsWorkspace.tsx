import type { ReactNode } from 'react'
import { Badge, Group, Stack, Tabs, Text } from '@mantine/core'
import { PageHeader } from '../../components/ui/PageHeader'

export type ProjectTab = 'overview' | 'applications' | 'changes' | 'rules'

type ProjectsWorkspaceProps = {
  projectName?: string
  projectDescription?: string
  projectNamespace?: string
  projectRole?: string
  projectActions?: ReactNode
  projectCatalog: ReactNode
  projectTab: ProjectTab
  onProjectTabChange: (tab: ProjectTab) => void
  overview: ReactNode
  applications: ReactNode
  changes: ReactNode
  rules: ReactNode
}

export function ProjectsWorkspace({
  projectName,
  projectDescription,
  projectNamespace,
  projectRole,
  projectActions,
  projectCatalog,
  projectTab,
  onProjectTabChange,

  overview,
  applications,
  changes,
  rules,
}: ProjectsWorkspaceProps) {
  return (
    <Stack gap="lg">
      {projectCatalog}

      {projectName ? (
        <>
          <PageHeader
            eyebrow="프로젝트 운영"
            title={projectName}
            description={projectDescription || '선택한 프로젝트의 운영 상태와 변경 작업을 관리합니다.'}
            actions={projectActions}
          />

          <Group gap="xs">
            {projectNamespace ? (
              <Badge color="lagoon.6" variant="light" radius="sm">
                {projectNamespace}
              </Badge>
            ) : null}
            {projectRole ? (
              <Badge color="lagoon.8" variant="light" radius="sm">
                {projectRole}
              </Badge>
            ) : null}
          </Group>

          <Tabs
            value={projectTab}
            onChange={(value) => {
              if (value) {
                onProjectTabChange(value as ProjectTab)
              }
            }}
            color="lagoon.6"
            variant="outline"
            radius="md"
          >
            <Tabs.List>
              <Tabs.Tab value="overview">개요</Tabs.Tab>
              <Tabs.Tab value="applications">애플리케이션</Tabs.Tab>
              <Tabs.Tab value="changes">변경 요청</Tabs.Tab>
              <Tabs.Tab value="rules">운영 규칙</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="overview" pt="md">
              {overview}
            </Tabs.Panel>
            <Tabs.Panel value="applications" pt="md">
              {applications}
            </Tabs.Panel>
            <Tabs.Panel value="changes" pt="md">
              {changes}
            </Tabs.Panel>
            <Tabs.Panel value="rules" pt="md">
              {rules}
            </Tabs.Panel>
          </Tabs>
        </>
      ) : (
        <Text c="lagoon.4">프로젝트를 선택하면 상세 운영 화면이 열립니다.</Text>
      )}
    </Stack>
  )
}
