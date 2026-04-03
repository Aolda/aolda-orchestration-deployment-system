import type { ChangeEvent } from 'react'
import { useRef, useState } from 'react'
import {
  Stepper,
  Button,
  Group,
  TextInput,
  Textarea,
  NumberInput,
  Select,
  Stack,
  Text,
  Card,
} from '@mantine/core'
export type CreateFormState = {
  name: string
  description: string
  image: string
  servicePort: number
  deploymentStrategy: 'Rollout' | 'Canary'
  environment: string
  secrets: { key: string; value: string }[]
}

type ApplicationWizardProps = {
  environments: { id: string; name: string }[]
  allowedStrategies: readonly string[]
  initialState: CreateFormState
  onSubmit: (state: CreateFormState) => Promise<void>
  onCancel: () => void
  submitting: boolean
}

export function ApplicationWizard({
  environments,
  allowedStrategies,
  initialState,
  onSubmit,
  onCancel,
  submitting,
}: ApplicationWizardProps) {
  const [active, setActive] = useState(0)
  const [form, setForm] = useState<CreateFormState>(initialState)
  const [envBulkText, setEnvBulkText] = useState('')
  const [envBulkMessage, setEnvBulkMessage] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const nextStep = () => setActive((current) => (current < 3 ? current + 1 : current))
  const prevStep = () => setActive((current) => (current > 0 ? current - 1 : current))

  const handleFinish = () => {
    onSubmit(form)
  }

  const updateForm = (updates: Partial<CreateFormState>) => {
    setForm((current) => ({ ...current, ...updates }))
  }

  const nextStrategy = (value: string | null): CreateFormState['deploymentStrategy'] =>
    value === 'Canary' ? 'Canary' : 'Rollout'

  const applyParsedSecrets = (text: string) => {
    const parsed = parseEnvEntries(text)
    if (parsed.length === 0) {
      setEnvBulkMessage('.env 형식에서 읽을 수 있는 항목이 없습니다.')
      return
    }
    updateForm({ secrets: parsed })
    setEnvBulkMessage(`${parsed.length}개 항목을 가져왔습니다.`)
  }

  const handleImportEnvFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    const text = await file.text()
    setEnvBulkText(text)
    applyParsedSecrets(text)
    event.target.value = ''
  }

  return (
    <Card shadow="sm" p="lg" radius="md" withBorder className="glass-panel">
      <Stepper active={active} onStepClick={setActive} color="lagoon.6" mb="xl">
        <Stepper.Step label="기본 정보" description="애플리케이션 메타데이터">
          <Stack gap="md" mt="md">
            <Text size="sm" c="dimmed">
              레포지토리는 public 또는 private 방식으로 연결할 수 있습니다. private 레포지토리는 PAT 기준으로 접근하며,
              배포 대상 서비스는 <Text span fw={700} ff="monospace">aolda.deploy.json</Text> 형식의 설정 파일을 기준으로 읽습니다.
              레포지토리 연결 정보는 프로젝트 설정에서 관리합니다.
            </Text>
            <TextInput
              label="애플리케이션 이름"
              placeholder="예: payment-api"
              required
              value={form.name}
              onChange={(e) => updateForm({ name: e.target.value })}
            />
            <Textarea
              label="설명"
              placeholder="애플리케이션의 역할과 책임"
              value={form.description}
              onChange={(e) => updateForm({ description: e.target.value })}
            />
            <TextInput
              label="이미지"
              placeholder="예: ghcr.io/aolda/payment-api:1.0.0"
              required
              value={form.image}
              onChange={(e) => updateForm({ image: e.target.value })}
            />
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="배포 설정" description="네트워크 및 환경">
          <Stack gap="md" mt="md">
            <NumberInput
              label="서비스 포트"
              required
              value={form.servicePort}
              onChange={(val) => updateForm({ servicePort: Number(val) || 8080 })}
            />
            <Select
              label="배포 전략"
              data={allowedStrategies}
              value={form.deploymentStrategy}
              onChange={(val) => updateForm({ deploymentStrategy: nextStrategy(val) })}
            />
            <Select
              label="배포 환경"
              data={environments.map((e) => ({ value: e.id, label: e.name }))}
              value={form.environment}
              onChange={(val) => updateForm({ environment: val || '' })}
            />
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="환경 변수/비밀값" description="Secrets 주입">
          <Stack gap="md" mt="md">
            <Text size="sm" c="dimmed">
              이곳에 입력된 값은 HashiCorp Vault에 저장되며 안전하게 파드에 마운트됩니다.
            </Text>
            <Textarea
              label=".env 일괄 입력"
              placeholder={'예:\nDB_HOST=db.internal\nDB_PASSWORD=secret-value\nAPI_KEY="abc123"'}
              minRows={6}
              value={envBulkText}
              onChange={(e) => setEnvBulkText(e.target.value)}
            />
            <Group>
              <Button
                variant="light"
                onClick={() => applyParsedSecrets(envBulkText)}
              >
                .env 내용 적용
              </Button>
              <Button
                variant="default"
                onClick={() => fileInputRef.current?.click()}
              >
                .env 파일 가져오기
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".env,text/plain"
                style={{ display: 'none' }}
                onChange={handleImportEnvFile}
              />
            </Group>
            <Text size="xs" c="dimmed">
              `KEY=value`, `export KEY=value`, 주석(`#`) 형식을 자동으로 파싱합니다. 같은 키가 여러 번 나오면 마지막 값을 사용합니다.
            </Text>
            {envBulkMessage ? (
              <Text size="sm" c="dimmed">{envBulkMessage}</Text>
            ) : null}
            {form.secrets.map((secret, index: number) => (
              <Group key={index} grow align="flex-end">
                <TextInput
                  label={index === 0 ? '키 (Key)' : undefined}
                  placeholder="예: DB_PASSWORD"
                  value={secret.key}
                  onChange={(e) => {
                    const newSecrets = [...form.secrets]
                    newSecrets[index].key = e.target.value
                    updateForm({ secrets: newSecrets })
                  }}
                />
                <TextInput
                  label={index === 0 ? '값 (Value)' : undefined}
                  placeholder="비밀값"
                  type="password"
                  value={secret.value}
                  onChange={(e) => {
                    const newSecrets = [...form.secrets]
                    newSecrets[index].value = e.target.value
                    updateForm({ secrets: newSecrets })
                  }}
                />
                <Button
                  color="red"
                  variant="light"
                  onClick={() => {
                    const newSecrets = form.secrets.filter((_, i: number) => i !== index)
                    updateForm({ secrets: newSecrets })
                  }}
                  disabled={form.secrets.length === 1}
                >
                  삭제
                </Button>
              </Group>
            ))}
            <Button
              variant="outline"
              onClick={() => updateForm({ secrets: [...form.secrets, { key: '', value: '' }] })}
            >
              + 추가
            </Button>
          </Stack>
        </Stepper.Step>

        <Stepper.Completed>
          <Stack gap="md" mt="md">
            <Text fw={500} size="lg">최종 검토</Text>
            <Text c="dimmed">아래 정보를 확인하고 애플리케이션 생성을 진행합니다.</Text>
            <Card withBorder bg="gray.0">
              <Group justify="space-between" mb="xs">
                <Text fw={700}>이름:</Text>
                <Text>{form.name || '(없음)'}</Text>
              </Group>
              <Group justify="space-between" mb="xs">
                <Text fw={700}>이미지:</Text>
                <Text>{form.image || '(없음)'}</Text>
              </Group>
              <Group justify="space-between" mb="xs">
                <Text fw={700}>환경:</Text>
                <Text>{environments.find(e => e.id === form.environment)?.name || form.environment}</Text>
              </Group>
              <Group justify="space-between">
                <Text fw={700}>전략:</Text>
                <Text>{form.deploymentStrategy}</Text>
              </Group>
            </Card>
          </Stack>
        </Stepper.Completed>
      </Stepper>

      <Group justify="flex-end" mt="xl">
        <Button variant="default" onClick={onCancel}>취소</Button>
        {active !== 0 && (
          <Button variant="default" onClick={prevStep}>
            이전
          </Button>
        )}
        {active < 3 ? (
          <Button onClick={nextStep} color="lagoon.6">다음 단계</Button>
        ) : (
          <Button onClick={handleFinish} loading={submitting} color="lagoon.8">설정 완료 및 생성</Button>
        )}
      </Group>
    </Card>
  )
}

function parseEnvEntries(text: string): { key: string; value: string }[] {
  const byKey = new Map<string, string>()

  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) {
      continue
    }

    const normalized = line.startsWith('export ') ? line.slice(7).trim() : line
    const separatorIndex = normalized.indexOf('=')
    if (separatorIndex <= 0) {
      continue
    }

    const key = normalized.slice(0, separatorIndex).trim()
    if (!key) {
      continue
    }

    let value = normalized.slice(separatorIndex + 1).trim()
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1)
    }

    value = value
      .replace(/\\n/g, '\n')
      .replace(/\\r/g, '\r')
      .replace(/\\t/g, '\t')
      .replace(/\\"/g, '"')
      .replace(/\\'/g, "'")

    byKey.set(key, value)
  }

  return Array.from(byKey.entries()).map(([key, value]) => ({ key, value }))
}
