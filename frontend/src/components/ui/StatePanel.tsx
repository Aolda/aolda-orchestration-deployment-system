import type { ReactNode } from 'react'
import { Group, Loader, Stack, Text, ThemeIcon } from '@mantine/core'
import {
  IconAlertTriangle,
  IconBan,
  IconFileSearch,
  IconInfoCircle,
  IconMoodEmpty,
} from '@tabler/icons-react'
import { SurfaceCard } from './SurfaceCard'

export type StateKind = 'loading' | 'empty' | 'partial' | 'error' | 'forbidden'

type StatePanelProps = {
  kind: StateKind
  title: string
  description: string
  action?: ReactNode
  withCard?: boolean
}

export function StatePanel({
  kind,
  title,
  description,
  action,
  withCard = true,
}: StatePanelProps) {
  const content = (
    <Group
      align="flex-start"
      gap="md"
      style={{
        padding: withCard ? 0 : '4px 0',
      }}
    >
      <StateIcon kind={kind} />
      <Stack gap={6} style={{ flex: 1 }}>
        <Text fw={800} c="lagoon.9">
          {title}
        </Text>
        <Text size="sm" c="lagoon.4">
          {description}
        </Text>
        {action}
      </Stack>
    </Group>
  )

  if (!withCard) {
    return content
  }

  return <SurfaceCard>{content}</SurfaceCard>
}

function StateIcon({ kind }: { kind: StateKind }) {
  if (kind === 'loading') {
    return (
      <ThemeIcon color="lagoon.6" variant="light" radius="xl" size={42}>
        <Loader size={18} color="var(--mantine-color-lagoon-6)" />
      </ThemeIcon>
    )
  }

  const config = {
    empty: {
      color: 'gray',
      icon: <IconMoodEmpty size={18} />,
    },
    partial: {
      color: 'yellow',
      icon: <IconInfoCircle size={18} />,
    },
    error: {
      color: 'red',
      icon: <IconAlertTriangle size={18} />,
    },
    forbidden: {
      color: 'violet',
      icon: <IconBan size={18} />,
    },
  } satisfies Record<Exclude<StateKind, 'loading'>, { color: string; icon: ReactNode }>

  const current = config[kind]

  return (
    <ThemeIcon color={current.color} variant="light" radius="xl" size={42}>
      {current.icon ?? <IconFileSearch size={18} />}
    </ThemeIcon>
  )
}
