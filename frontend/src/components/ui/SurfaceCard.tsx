import type { ReactNode } from 'react'
import { Paper } from '@mantine/core'

type SurfaceCardProps = {
  children: ReactNode
}

export function SurfaceCard({ children }: SurfaceCardProps) {
  return (
    <Paper
      radius="xl"
      p="lg"
      withBorder
      style={{
        borderColor: 'var(--portal-border)',
        background: 'rgba(255,255,255,0.94)',
      }}
    >
      {children}
    </Paper>
  )
}
