import type { ReactNode } from 'react'
import { Stack, Text } from '@mantine/core'
import { PageHeader } from '../../components/ui/PageHeader'

type ChangesWorkspaceProps = {
  content: ReactNode
}

export function ChangesWorkspace({ content }: ChangesWorkspaceProps) {
  return (
    <Stack gap="lg">
      <PageHeader
        eyebrow="변경 요청"
        title="변경 요청"
        description="생성된 변경 요청을 제출, 승인, 반영하는 작업 공간입니다."
      />
      <Text c="lagoon.4">
        현재 화면은 변경 요청 목록 없이 선택된 변경 객체를 중심으로 동작합니다. 생성 흐름에서 연결
        된 변경 요청을 여기서 이어서 처리합니다.
      </Text>
      {content}
    </Stack>
  )
}
