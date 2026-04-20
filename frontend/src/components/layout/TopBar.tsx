import type { ReactNode } from 'react'
import { Badge, Group, Text } from '@mantine/core'
import classes from '../../app/layout/AppShell.module.css'

type TopBarProps = {
  breadcrumbs: string[]
  title: string
  description: string
  actions?: ReactNode
  metaBadges?: ReactNode
  userLabel?: string
  roleLabel?: string
}

export function TopBar({
  breadcrumbs,
  title,
  description,
  actions,
  metaBadges,
  userLabel,
  roleLabel,
}: TopBarProps) {
  return (
    <div className={classes.topbar}>
      <div className={classes.topbarInner}>
        <div className={classes.topbarMeta}>
          <div className={classes.breadcrumbs}>
            {breadcrumbs.map((crumb, index) => (
              <span
                key={`${crumb}-${index}`}
                className={index === breadcrumbs.length - 1 ? classes.crumbCurrent : undefined}
              >
                {crumb}
              </span>
            ))}
          </div>
          <div className={classes.topbarTitle}>{title}</div>
          <Text className={classes.topbarBody}>{description}</Text>
          {metaBadges ? <Group gap="xs" className={classes.topbarMetaBadges}>{metaBadges}</Group> : null}
        </div>

        <div className={classes.topbarActions}>
          {userLabel ? (
            <Badge color="lagoon.6" variant="light" radius="sm">
              {userLabel}
            </Badge>
          ) : null}
          {roleLabel ? (
            <Badge color="lagoon.8" variant="light" radius="sm">
              {roleLabel}
            </Badge>
          ) : null}
          <Group gap="sm">{actions}</Group>
        </div>
      </div>
    </div>
  )
}
