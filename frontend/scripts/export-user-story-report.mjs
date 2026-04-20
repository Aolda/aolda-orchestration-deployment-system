import { mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = resolve(fileURLToPath(new URL('..', import.meta.url)))
const repoRoot = resolve(frontendDir, '..')
const storiesPath = join(frontendDir, 'user-stories.json')
const srcDir = join(frontendDir, 'src')
const markdownPath = join(repoRoot, 'docs', 'frontend-user-story-report.md')
const jsonDir = join(frontendDir, 'public', 'reports')
const jsonPath = join(jsonDir, 'frontend-user-story-report.json')

const { critical } = JSON.parse(readFileSync(storiesPath, 'utf8'))
const testFiles = []

collectTestFiles(srcDir)

const generatedAt = new Date().toISOString()

const rows = critical.map((story) => {
  const token = `[${story.id}]`
  const matchedFiles = testFiles.filter((filePath) => readFileSync(filePath, 'utf8').includes(token))

  return {
    ...story,
    covered: matchedFiles.length > 0,
    files: matchedFiles.map((filePath) => relative(repoRoot, filePath)),
  }
})

const summary = {
  total: rows.length,
  covered: rows.filter((row) => row.covered).length,
  uncovered: rows.filter((row) => !row.covered).length,
}

const report = {
  generatedAt,
  summary,
  stories: rows,
}

mkdirSync(jsonDir, { recursive: true })
writeFileSync(jsonPath, `${JSON.stringify(report, null, 2)}\n`)
writeFileSync(markdownPath, buildMarkdown(report))

console.log(`Markdown report written to ${relative(repoRoot, markdownPath)}`)
console.log(`JSON report written to ${relative(repoRoot, jsonPath)}`)

function buildMarkdown(currentReport) {
  const lines = [
    '# Frontend User Story Report',
    '',
    `Generated at: \`${currentReport.generatedAt}\``,
    '',
    `* Total critical stories: \`${currentReport.summary.total}\``,
    `* Covered: \`${currentReport.summary.covered}\``,
    `* Uncovered: \`${currentReport.summary.uncovered}\``,
    '',
  ]

  for (const story of currentReport.stories) {
    lines.push(`## ${story.id}`)
    lines.push('')
    lines.push(`* Screen: \`${story.screen}\``)
    lines.push(`* Persona: \`${story.persona}\``)
    lines.push(`* Status: \`${story.covered ? 'covered' : 'missing'}\``)
    lines.push(`* Story: ${story.story}`)
    if (story.files.length > 0) {
      lines.push(`* Tests: ${story.files.map((file) => `\`${file}\``).join(', ')}`)
    } else {
      lines.push('* Tests: none')
    }
    lines.push('')
  }

  return `${lines.join('\n')}\n`
}

function collectTestFiles(directory) {
  for (const entry of readdirSync(directory)) {
    const nextPath = join(directory, entry)
    const stats = statSync(nextPath)
    if (stats.isDirectory()) {
      collectTestFiles(nextPath)
      continue
    }

    if (/\.(test|spec)\.(ts|tsx)$/.test(entry)) {
      testFiles.push(nextPath)
    }
  }
}
