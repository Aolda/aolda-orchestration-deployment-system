import { useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Group,
  NumberInput,
  ScrollArea,
  SimpleGrid,
  Stack,
  Switch,
  Table,
  Text,
} from '@mantine/core'
import { IconAlertTriangle, IconLock } from '@tabler/icons-react'

import classes from '../../App.module.css'
import { showRollbackPolicyControls, supportedDeploymentStrategies } from '../../app/appConfig'
import { HelpTooltipLabel } from '../../components/ui/HelpTooltipLabel'
import type { EnvironmentSummary, ProjectPolicy, ProjectSummary } from '../../types/api'

type ProjectSettingsPanelProps = {
  project: ProjectSummary | undefined
  environments: EnvironmentSummary[]
  projectPolicy: ProjectPolicy | null
  canEditPolicies: boolean
  savingPolicies: boolean
  onSavePolicies: (policy: ProjectPolicy) => void
  applicationCount: number
  canDeleteProject: boolean
  isProtectedProject: boolean
  deletingProject: boolean
  pendingProjectDelete: boolean
  onRequestProjectDelete: () => void
  onCancelProjectDelete: () => void
  onConfirmProjectDelete: () => void
}

export function ProjectSettingsPanel({
  project,
  environments,
  projectPolicy,
  canEditPolicies,
  savingPolicies,
  onSavePolicies,
  applicationCount,
  canDeleteProject,
  isProtectedProject,
  deletingProject,
  pendingProjectDelete,
  onRequestProjectDelete,
  onCancelProjectDelete,
  onConfirmProjectDelete,
}: ProjectSettingsPanelProps) {
  const [policyDraft, setPolicyDraft] = useState<ProjectPolicy | null>(() => cloneProjectPolicy(projectPolicy))

  return (
    <Stack gap="xl" className={classes.projectSettingsPanel}>
      <div className={classes.surfaceCard}>
        <Text className={classes.sectionEyebrow} mb="md">기본 정보</Text>
        <Alert color="gray" variant="light" radius="md" mb="md">
          프로젝트 이름은 영문 소문자 slug 규칙으로 생성되며, 프로젝트 ID와 Kubernetes namespace도 같은 값을 사용합니다. 이 식별자는 생성 후 변경할 수 없습니다.
        </Alert>
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
          <div>
            <HelpTooltipLabel label="프로젝트 이름" description="프로젝트 식별에 사용하는 영문 slug이며, 내부 ID와 동일하게 동작합니다." />
            <Text size="sm" fw={800}>{project?.name || '-'}</Text>
          </div>
          <div>
            <HelpTooltipLabel label="네임스페이스" description="배포 리소스와 애플리케이션을 논리적으로 묶는 기본 공간입니다." />
            <Text size="sm" fw={800}>{project?.namespace || '-'}</Text>
          </div>
        </SimpleGrid>
      </div>

      <div className={classes.surfaceCard}>
        <Text className={classes.sectionEyebrow} mb="md">운영 환경</Text>
        {environments.length > 0 ? (
          <ScrollArea className={classes.projectSettingsTableScroll} offsetScrollbars>
            <Table striped highlightOnHover className={classes.projectSettingsTable} miw={640}>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>
                    <HelpTooltipLabel label="이름" description="배포와 운영에서 구분하는 환경 이름입니다. 예: dev, staging, prod" />
                  </Table.Th>
                  <Table.Th>
                    <HelpTooltipLabel label="클러스터" description="이 운영 환경이 실제로 반영되는 대상 클러스터입니다." />
                  </Table.Th>
                  <Table.Th>
                    <HelpTooltipLabel label="반영 방식" description="직접 반영이면 바로 적용되고, 변경 요청이면 승인 흐름을 거쳐 반영됩니다." />
                  </Table.Th>
                  <Table.Th>
                    <HelpTooltipLabel label="기본" description="새 애플리케이션 생성이나 배포 시 기본 선택으로 잡히는 환경인지 표시합니다." />
                  </Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {environments.map((environment) => (
                  <Table.Tr key={environment.id}>
                    <Table.Td>{environment.name}</Table.Td>
                    <Table.Td>{environment.clusterId}</Table.Td>
                    <Table.Td>{environment.writeMode === 'pull_request' ? '변경 요청' : '직접 반영'}</Table.Td>
                    <Table.Td>
                      <Badge color={environment.default ? 'lagoon.6' : 'gray'} variant="light">
                        {environment.default ? '기본' : '일반'}
                      </Badge>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </ScrollArea>
        ) : (
          <Text size="sm" c="dimmed">운영 환경 정보를 아직 불러오지 못했습니다.</Text>
        )}
      </div>

      <div className={classes.surfaceCard}>
        <Text className={classes.sectionEyebrow} mb="md">배포 정책</Text>
        {policyDraft ? (
          <Stack gap="md">
            <Text size="sm" c="dimmed">
              아래 정책 값은 이 화면에서 바로 수정할 수 있습니다. 저장하면 현재 프로젝트의 운영 가드레일에 즉시 반영됩니다.
            </Text>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              <div>
                <HelpTooltipLabel label="최소 복제본 수" description="서비스가 유지해야 하는 최소 replica 기준값입니다." />
                <NumberInput
                  min={0}
                  value={policyDraft.minReplicas}
                  onChange={(value) => {
                    const nextValue = typeof value === 'number' ? value : Number(value || 0)
                    setPolicyDraft((current) => (current ? { ...current, minReplicas: Number.isFinite(nextValue) ? nextValue : 0 } : current))
                  }}
                  disabled={!canEditPolicies}
                />
              </div>
              <div>
                <HelpTooltipLabel label="프로브 필수" description="liveness, readiness 같은 헬스체크 설정을 필수로 요구하는지 나타냅니다." />
                <Switch
                  checked={policyDraft.requiredProbes}
                  onChange={(event) => {
                    const checked = event.currentTarget.checked
                    setPolicyDraft((current) => (current ? { ...current, requiredProbes: checked } : current))
                  }}
                  disabled={!canEditPolicies}
                />
              </div>
              <div>
                <HelpTooltipLabel label="운영 환경 변경 요청 필수" description="운영 성격 환경에는 직접 반영 대신 승인 기반 변경 요청이 필요한지 보여줍니다." />
                <Switch
                  checked={policyDraft.prodPRRequired}
                  onChange={(event) => {
                    const checked = event.currentTarget.checked
                    setPolicyDraft((current) => (current ? { ...current, prodPRRequired: checked } : current))
                  }}
                  disabled={!canEditPolicies}
                />
              </div>
              {showRollbackPolicyControls ? (
                <div>
                  <HelpTooltipLabel label="자동 롤백" description="배포 이상 징후가 생기면 이전 안정 버전으로 자동 복구할지 나타냅니다." />
                  <Switch
                    checked={policyDraft.autoRollbackEnabled}
                    onChange={(event) => {
                      const checked = event.currentTarget.checked
                      setPolicyDraft((current) => (current ? { ...current, autoRollbackEnabled: checked } : current))
                    }}
                    disabled={!canEditPolicies}
                  />
                </div>
              ) : null}
              <div>
                <HelpTooltipLabel label="허용 환경" description="이 프로젝트가 배포 대상으로 사용할 수 있는 운영 환경 목록입니다." />
                <Text size="sm" fw={800}>{policyDraft.allowedEnvironments.join(', ') || '-'}</Text>
                <Text size="xs" c="dimmed">현재 플랫폼 기본값으로 고정되어 있으며 이 화면에서 수정하지 않습니다.</Text>
              </div>
              <div>
                <HelpTooltipLabel label="허용 배포 전략" description="이 프로젝트에서 선택 가능한 배포 방식 목록입니다." />
                <Text size="sm" fw={800}>{supportedDeploymentStrategies.join(', ')}</Text>
                <Text size="xs" c="dimmed">현재 프론트에서는 Rollout만 지원하도록 고정되어 있습니다.</Text>
              </div>
              <div style={{ gridColumn: '1 / -1' }}>
                <HelpTooltipLabel label="허용 클러스터 대상" description="이 프로젝트가 배포될 수 있는 클러스터 범위를 정의합니다." />
                <Text size="sm" fw={800}>{policyDraft.allowedClusterTargets.join(', ') || '-'}</Text>
                <Text size="xs" c="dimmed">현재 플랫폼 기본값으로 고정되어 있으며 이 화면에서 수정하지 않습니다.</Text>
              </div>
            </SimpleGrid>

            <Group justify="space-between" align="center">
              {!canEditPolicies ? (
                <Text size="sm" c="dimmed">admin 역할만 정책 변경을 저장할 수 있습니다.</Text>
              ) : (
                <Text size="sm" c="dimmed">허용 환경, 배포 전략, 클러스터 대상은 현재 플랫폼 기본값으로 고정됩니다.</Text>
              )}
              <Group>
                <Button
                  variant="default"
                  disabled={!canEditPolicies}
                  onClick={() => {
                    setPolicyDraft(cloneProjectPolicy(projectPolicy))
                  }}
                >
                  되돌리기
                </Button>
                <Button
                  color="lagoon.6"
                  loading={savingPolicies}
                  disabled={!canEditPolicies || !policyDraft}
                  onClick={() => {
                    if (policyDraft) {
                      onSavePolicies(policyDraft)
                    }
                  }}
                >
                  정책 저장
                </Button>
              </Group>
            </Group>
          </Stack>
        ) : (
          <Text size="sm" c="dimmed">프로젝트 정책 정보를 아직 불러오지 못했습니다.</Text>
        )}
      </div>

      <div className={classes.surfaceCard}>
        <Group justify="space-between" align="start" mb="md">
          <div>
            <Text className={classes.sectionEyebrow}>위험 구역</Text>
            <Text fw={800}>프로젝트 삭제</Text>
            <Text size="sm" c="dimmed" mt={6}>
              프로젝트 카탈로그 엔트리와 애플리케이션 {applicationCount}개, 연결된 앱 시크릿을 함께 정리합니다.
            </Text>
          </div>
          <Badge color={isProtectedProject ? 'gray' : canDeleteProject ? 'red' : 'gray'} variant="light">
            {isProtectedProject ? '보호됨' : canDeleteProject ? '삭제 가능' : '권한 필요'}
          </Badge>
        </Group>

        {isProtectedProject ? (
          <Alert color="gray" radius="md" icon={<IconLock size={16} />}>
            공용 프로젝트는 보호 대상이라 삭제할 수 없습니다.
          </Alert>
        ) : pendingProjectDelete ? (
          <Alert color="red" radius="md" icon={<IconAlertTriangle size={16} />}>
            <Text size="sm">
              이 작업은 되돌릴 수 없습니다. 프로젝트 자체와 하위 애플리케이션 운영 흔적이 함께 정리됩니다.
            </Text>
            <Group mt="md">
              <Button color="red" loading={deletingProject} onClick={onConfirmProjectDelete}>
                프로젝트 삭제 확인
              </Button>
              <Button variant="default" onClick={onCancelProjectDelete}>
                취소
              </Button>
            </Group>
          </Alert>
        ) : (
          <Stack gap="xs">
            <Button color="red" variant="light" disabled={!canDeleteProject} onClick={onRequestProjectDelete}>
              프로젝트 삭제
            </Button>
            {!canDeleteProject ? (
              <Text size="sm" c="dimmed">platform admin만 프로젝트 삭제를 실행할 수 있습니다.</Text>
            ) : null}
          </Stack>
        )}
      </div>
    </Stack>
  )
}

function cloneProjectPolicy(policy: ProjectPolicy | null): ProjectPolicy | null {
  if (!policy) return null
  return {
    ...policy,
    allowedEnvironments: [...policy.allowedEnvironments],
    allowedDeploymentStrategies: [...supportedDeploymentStrategies],
    allowedClusterTargets: [...policy.allowedClusterTargets],
  }
}
