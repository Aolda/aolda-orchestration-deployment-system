import type { ReactNode } from 'react'
import { AppShell as MantineAppShell, ScrollArea, Text } from '@mantine/core'
import type { GlobalSection } from './navigation'
import classes from './AppShell.module.css'
import { SidebarNav } from '../../components/navigation/SidebarNav'
import { TopBar } from '../../components/layout/TopBar'

type SidebarProject = {
  id: string
  name: string
  namespace: string
  role: string
}

type AppShellProps = {
  activeSection: GlobalSection
  onSectionChange: (section: GlobalSection) => void
  visibleSections?: GlobalSection[]
  breadcrumbs: string[]
  title: string
  description: string
  topBarActions?: ReactNode
  metaBadges?: ReactNode
  secondaryNav?: ReactNode
  userLabel?: string
  roleLabel?: string
  projects?: SidebarProject[]
  selectedProjectId?: string | null
  onProjectSelect?: (projectId: string) => void
  canCreateProject?: boolean
  onCreateProject?: () => void
  children: ReactNode
}

export function AppShell({
  activeSection,
  onSectionChange,
  visibleSections,
  breadcrumbs,
  title,
  description,
  topBarActions,
  metaBadges,
  secondaryNav,
  userLabel,
  roleLabel,
  projects,
  selectedProjectId,
  onProjectSelect,
  canCreateProject,
  onCreateProject,
  children,
}: AppShellProps) {
  return (
    <MantineAppShell
      layout="alt"
      navbar={{ width: 248, breakpoint: 'sm' }}
      padding={0}
      className={classes.shell}
    >
      <MantineAppShell.Navbar className={classes.navbar}>
        <div className={classes.navbarInner}>
          <div className={classes.brandBlock}>
            <div className={classes.brandKicker}>AODS</div>
            <div className={classes.brandTitle}>내부 배포 운영 플랫폼</div>
            <Text className={classes.brandBody}>
              AODS는 배포 대상, 운영 규칙, 변경 흐름을 한 화면에서 관리하는 내부 배포 플랫폼입니다.
            </Text>
          </div>

          <ScrollArea type="never" flex={1}>
            <SidebarNav
              activeSection={activeSection}
              onSectionChange={onSectionChange}
              visibleSections={visibleSections}
              projects={projects}
              selectedProjectId={selectedProjectId}
              onProjectSelect={onProjectSelect}
              canCreateProject={canCreateProject}
              onCreateProject={onCreateProject}
            />
            {secondaryNav && (
              <div className={classes.secondaryNavContainer}>
                {secondaryNav}
              </div>
            )}
          </ScrollArea>
        </div>
      </MantineAppShell.Navbar>

      <MantineAppShell.Main className={classes.main}>
        <div className={classes.content}>
          <TopBar
            breadcrumbs={breadcrumbs}
            title={title}
            description={description}
            actions={topBarActions}
            metaBadges={metaBadges}
            userLabel={userLabel}
            roleLabel={roleLabel}
          />
          {children}
        </div>
      </MantineAppShell.Main>
    </MantineAppShell>
  )
}
