import { useState, type FormEvent } from 'react'
import { Alert, Button, Divider, PasswordInput, Stack, Text, TextInput } from '@mantine/core'
import { IconLock, IconUser } from '@tabler/icons-react'

import classes from '../../App.module.css'

const localLoginUsername = 'admin'
const localLoginPassword = 'qwe1356@'

type LoginMode = 'oidc' | 'emergency' | 'password'

type LoginFormProps = {
  oidcEnabled: boolean
  allowEmergencyLogin: boolean
  onLogin: () => Promise<void>
  onEmergencyLogin: (username: string, password: string) => Promise<void>
}

export function LoginForm({ oidcEnabled, allowEmergencyLogin, onLogin, onEmergencyLogin }: LoginFormProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState<LoginMode | null>(null)

  const runLogin = async (mode: LoginMode) => {
    setSubmitting(mode)
    setError('')
    try {
      if (mode === 'emergency') {
        await onEmergencyLogin(username, password)
      } else {
        await onLogin()
      }
    } catch (loginError) {
      setSubmitting(null)
      setError(loginError instanceof Error ? loginError.message : '로그인을 시작하지 못했습니다.')
    }
  }

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (oidcEnabled) {
      void runLogin('emergency')
      return
    }

    if (username === localLoginUsername && password === localLoginPassword) {
      void runLogin('password')
      return
    }

    setError('아이디 또는 비밀번호가 올바르지 않습니다.')
  }

  return (
    <div className={classes.authShell}>
      <div className={classes.loginCard}>
        <Stack gap="xl">
          <div style={{ textAlign: 'center' }}>
            <Text
              fw={900}
              style={{
                fontSize: '3rem',
                letterSpacing: '-0.06em',
                lineHeight: 1,
              }}
            >
              AODS
            </Text>
            <Text size="sm" c="dimmed" mt={8}>
              내부 배포 관리 플랫폼
            </Text>
          </div>

          {oidcEnabled && allowEmergencyLogin ? (
            <Stack gap="lg">
              <form onSubmit={handleSubmit} autoComplete="off">
                <Stack gap="md">
                  {error ? <Alert color="red">{error}</Alert> : null}
                  <TextInput
                    label="아이디"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    required
                    leftSection={<IconUser size={16} />}
                    name="aods-login-username"
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                  />
                  <PasswordInput
                    label="비밀번호"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    required
                    leftSection={<IconLock size={16} />}
                    name="aods-login-password"
                    autoComplete="new-password"
                  />
                  <Button fullWidth size="md" color="lagoon.6" type="submit" mt="xs" loading={submitting === 'emergency'}>
                    로그인
                  </Button>
                </Stack>
              </form>

              <Divider label="또는" labelPosition="center" />

              <Stack gap="xs">
                <Button fullWidth size="md" variant="default" onClick={() => void runLogin('oidc')} loading={submitting === 'oidc'}>
                  Keycloak으로 로그인
                </Button>
              </Stack>
            </Stack>
          ) : oidcEnabled ? (
            <Stack gap="md">
              <Alert color="blue" variant="light">
                사내 SSO(Keycloak)로 로그인한 뒤 `aods` 클라이언트 권한을 기준으로 포털 접근 범위를 결정합니다.
              </Alert>
              {error ? <Alert color="red">{error}</Alert> : null}
              <Button fullWidth size="md" color="lagoon.6" onClick={() => void runLogin('oidc')} loading={submitting === 'oidc'}>
                Keycloak로 로그인
              </Button>
            </Stack>
          ) : (
            <form onSubmit={handleSubmit} autoComplete="off">
              <Stack gap="md">
                <TextInput
                  label="아이디"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  required
                  leftSection={<IconUser size={16} />}
                  name="aods-login-username"
                  autoComplete="off"
                  autoCapitalize="none"
                  spellCheck={false}
                />
                <PasswordInput
                  label="비밀번호"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                  leftSection={<IconLock size={16} />}
                  name="aods-login-password"
                  autoComplete="new-password"
                />
                {error ? <Alert color="red">{error}</Alert> : null}
                <Button fullWidth size="md" color="lagoon.6" type="submit" mt="md" loading={submitting === 'password'}>
                  로그인
                </Button>
              </Stack>
            </form>
          )}
        </Stack>
      </div>
    </div>
  )
}
