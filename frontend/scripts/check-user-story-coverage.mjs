import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = resolve(fileURLToPath(new URL('..', import.meta.url)))
const storiesPath = join(frontendDir, 'user-stories.json')
const srcDir = join(frontendDir, 'src')

const { critical } = JSON.parse(readFileSync(storiesPath, 'utf8'))
const testFiles = []

collectTestFiles(srcDir)

if (testFiles.length === 0) {
  console.error('No frontend test files found under src/.')
  process.exit(1)
}

const missing = critical.filter((story) => {
  const storyToken = `[${story.id}]`
  return !testFiles.some((filePath) => readFileSync(filePath, 'utf8').includes(storyToken))
})

if (missing.length > 0) {
  console.error('Missing user-story test coverage for:')
  for (const story of missing) {
    console.error(`- ${story.id}: ${story.story}`)
  }
  process.exit(1)
}

console.log(`User story coverage OK: ${critical.length}/${critical.length}`)

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
