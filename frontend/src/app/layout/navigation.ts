export type GlobalSection = 'projects' | 'changes' | 'clusters' | 'me'

export const globalSections: Array<{ value: GlobalSection; label: string; description: string }> = [
  {
    value: 'projects',
    label: '프로젝트',
    description: '프로젝트와 애플리케이션 운영',
  },
  {
    value: 'clusters',
    label: '클러스터',
    description: '배포 대상 클러스터 카탈로그',
  },
  {
    value: 'me',
    label: '내 정보',
    description: '로그인 사용자와 접근 범위',
  },
]
