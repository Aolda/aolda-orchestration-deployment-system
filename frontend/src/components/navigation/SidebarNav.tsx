import { NavLink, Stack } from '@mantine/core'
import type { GlobalSection } from '../../app/layout/navigation'
import { globalSections } from '../../app/layout/navigation'

type SidebarNavProps = {
  activeSection: GlobalSection
  onSectionChange: (section: GlobalSection) => void
}

export function SidebarNav({ activeSection, onSectionChange }: SidebarNavProps) {
  return (
    <Stack gap="xs">
      {globalSections.map((section) => (
        <NavLink
          key={section.value}
          active={section.value === activeSection}
          label={section.label}
          description={section.description}
          variant="light"
          color="lagoon.6"
          onClick={() => onSectionChange(section.value)}
        />
      ))}
    </Stack>
  )
}
