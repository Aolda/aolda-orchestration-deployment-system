import type { ReactNode } from 'react'
import { Stack, Tabs, Text } from '@mantine/core'
import classes from './ProjectsWorkspace.module.css'

export type ProjectTab = 'applications' | 'monitoring' | 'rules'

type ProjectsWorkspaceProps = {
  projectName?: string
  projectDescription?: string
  projectNamespace?: string
  projectRole?: string
  projectActions?: ReactNode
  projectNotice?: ReactNode
  projectCatalog?: ReactNode
  projectTab: ProjectTab
  onProjectTabChange: (tab: ProjectTab) => void
  applications: ReactNode
  monitoring: ReactNode
  rules: ReactNode
}

export function ProjectsWorkspace(props: ProjectsWorkspaceProps) {
  const hasCatalog = Boolean(props.projectCatalog)

  return (
    <div className={hasCatalog ? classes.layout : classes.layoutSingle}>
      {hasCatalog ? <aside className={classes.catalogRail}>{props.projectCatalog}</aside> : null}

      <div className={hasCatalog ? classes.detailPanel : classes.detailPanelSolo}>
        {props.projectName ? (
          <>
            {props.projectNotice}

            <div className={classes.tabsCard}>
              <Tabs
                value={props.projectTab}
                onChange={(value) => {
                  if (value) {
                    props.onProjectTabChange(value as ProjectTab)
                  }
                }}
                keepMounted={false}
                color="lagoon.6"
                variant="outline"
                radius="md"
              >
                <Tabs.List>
                  <Tabs.Tab value="applications">애플리케이션</Tabs.Tab>
                  <Tabs.Tab value="monitoring">모니터링</Tabs.Tab>
                  <Tabs.Tab value="rules">운영 규칙</Tabs.Tab>
                </Tabs.List>

                <Tabs.Panel value="applications" pt="md">
                  {props.applications}
                </Tabs.Panel>
                <Tabs.Panel value="monitoring" pt="md">
                  {props.monitoring}
                </Tabs.Panel>
                <Tabs.Panel value="rules" pt="md">
                  {props.rules}
                </Tabs.Panel>
              </Tabs>
            </div>
          </>
        ) : (
          <div className={classes.emptyPanel}>
            <Stack gap="sm" align="center">
              <Text fw={800} size="lg" c="lagoon.9">
                프로젝트를 선택하면 상세 운영 화면이 열립니다.
              </Text>
              <Text className={classes.emptyBody}>
                좌측 사이드바의 프로젝트 목록에서 프로젝트를 고르면 애플리케이션, 모니터링, 운영 규칙 탭이 바로 이 영역에서 열립니다.
              </Text>
            </Stack>
          </div>
        )}
      </div>
    </div>
  )
}
