import { Badge, Button, NavLink, Stack, Text, UnstyledButton } from '@mantine/core'
import type { GlobalSection } from '../../app/layout/navigation'
import { globalSections } from '../../app/layout/navigation'
import classes from './SidebarNav.module.css'

type SidebarProject = {
  id: string
  name: string
  namespace: string
  role: string
}

type SidebarNavProps = {
  activeSection: GlobalSection
  onSectionChange: (section: GlobalSection) => void
  projects?: SidebarProject[]
  selectedProjectId?: string | null
  onProjectSelect?: (projectId: string) => void
  canCreateProject?: boolean
  onCreateProject?: () => void
}

export function SidebarNav({
  activeSection,
  onSectionChange,
  projects = [],
  selectedProjectId,
  onProjectSelect,
  canCreateProject = false,
  onCreateProject,
}: SidebarNavProps) {
  return (
    <Stack gap="xs">
      {globalSections.map((section) => (
        <div key={section.value}>
          <NavLink
            active={section.value === activeSection}
            label={section.label}
            description={section.description}
            variant="light"
            color="lagoon.6"
            onClick={() => onSectionChange(section.value)}
          />

          {section.value === 'projects' ? (
            <div className={classes.projectRail}>
              <Text className={classes.projectRailLabel}>접근 가능한 프로젝트</Text>
              {canCreateProject ? (
                <Button
                  variant="light"
                  color="lagoon.6"
                  fullWidth
                  radius="md"
                  className={classes.projectRailAction}
                  onClick={() => onCreateProject?.()}
                >
                  새 프로젝트
                </Button>
              ) : null}
              {projects.length > 0 ? (
                <Stack gap={8}>
                  {projects.map((project) => (
                    <UnstyledButton
                      key={project.id}
                      className={`${classes.projectItem} ${
                        selectedProjectId === project.id ? classes.projectItemActive : ''
                      }`}
                      onClick={() => onProjectSelect?.(project.id)}
                    >
                      <div className={classes.projectItemHeader}>
                        <Text className={classes.projectName}>{project.name}</Text>
                        <Badge color="lagoon.6" variant="light" radius="sm" size="xs">
                          {project.role}
                        </Badge>
                      </div>
                      <Text className={classes.projectMeta}>{project.namespace}</Text>
                    </UnstyledButton>
                  ))}
                </Stack>
              ) : (
                <Text className={classes.projectRailEmpty}>접근 가능한 프로젝트가 없습니다.</Text>
              )}
            </div>
          ) : null}
        </div>
      ))}
    </Stack>
  )
}
