import { Group, Text, Tooltip } from '@mantine/core'

type HelpTooltipLabelProps = {
  label: string
  description: string
}

export function HelpTooltipLabel({ label, description }: HelpTooltipLabelProps) {
  return (
    <Group gap={6} align="center" wrap="nowrap">
      <Text size="xs" c="dimmed" fw={700}>{label}</Text>
      <Tooltip label={description} multiline w={280} withArrow position="top-start">
        <Text
          component="span"
          size="xs"
          fw={800}
          c="lagoon.6"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: '18px',
            height: '18px',
            borderRadius: '999px',
            border: '1px solid #bfdbfe',
            background: '#eff6ff',
            cursor: 'help',
            flexShrink: 0,
          }}
        >
          ?
        </Text>
      </Tooltip>
    </Group>
  )
}
