import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = resolve(fileURLToPath(new URL('..', import.meta.url)))
const storiesPath = join(frontendDir, 'user-stories.json')
const srcDir = join(frontendDir, 'src')

const { critical } = JSON.parse(readFileSync(storiesPath, 'utf8'))
const testFiles = []

collectTestFiles(srcDir)

const rows = critical.map((story) => {
  const token = `[${story.id}]`
  const matchedFiles = testFiles.filter((filePath) => readFileSync(filePath, 'utf8').includes(token))

  return {
    ...story,
    covered: matchedFiles.length > 0,
    files: matchedFiles.map((filePath) => relative(frontendDir, filePath)),
  }
})

const coveredCount = rows.filter((row) => row.covered).length

console.log('Frontend User Story Report')
console.log(`Critical stories: ${rows.length}`)
console.log(`Covered: ${coveredCount}`)
console.log(`Uncovered: ${rows.length - coveredCount}`)
console.log('')

for (const row of rows) {
  console.log(`${row.covered ? '[OK]' : '[MISSING]'} ${row.id} | ${row.screen} | ${row.persona}`)
  console.log(`story: ${row.story}`)
  if (row.files.length > 0) {
    console.log(`tests: ${row.files.join(', ')}`)
  } else {
    console.log('tests: none')
  }
  console.log('')
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
