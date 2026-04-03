import type { ReactNode } from 'react'
import { Group, Text, Title } from '@mantine/core'

type PageHeaderProps = {
  eyebrow: string
  title: string
  description?: string
  actions?: ReactNode
}

export function PageHeader({ eyebrow, title, description, actions }: PageHeaderProps) {
  return (
    <Group justify="space-between" align="end" mb="md">
      <div>
        <Text size="xs" fw={700} tt="uppercase" c="lagoon.5" mb={6}>
          {eyebrow}
        </Text>
        <Title order={2} c="lagoon.9">
          {title}
        </Title>
        {description ? (
          <Text mt={6} c="lagoon.4">
            {description}
          </Text>
        ) : null}
      </div>
      {actions}
    </Group>
  )
}
