import { readFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

for (const scope of ['app', 'admin']) {
  const source = readFileSync(new URL(`../../cmd/${scope}/desc/${scope}.api`, import.meta.url), 'utf8')
  let output = 'export type Int64 = number | bigint\n\n'
  for (const type of source.matchAll(/^type (\w+) \{([^}]*?)\}/gm)) {
    const fields = []
    for (const line of type[2].split('\n')) {
      if (!line.trim() || line.trim().startsWith('//')) continue
      const field = /^\s*\w+\s+(\*?)(\[\])?(\w+)\s+`(?:json|form|path):"([^",]+)([^"]*)"`/.exec(line)
      if (!field) throw new Error(`Unsupported API field: ${line}`)
      const [, pointer, array, kind, name, options] = field
      const tsType = { int64: 'Int64', int32: 'number', string: 'string', bool: 'boolean' }[kind] ?? kind
      fields.push(`  ${name}${pointer || options.includes('optional') ? '?' : ''}: ${tsType}${array ? '[]' : ''}`)
    }
    output += `export type ${type[1]} = {\n${fields.join('\n')}\n}\n\n`
  }
  const destination = new URL(`../src/api/contracts/${scope}.ts`, import.meta.url)
  if (process.argv.includes('--check')) {
    if (readFileSync(destination, 'utf8') !== output) throw new Error(`Stale ${scope} contracts; run npm run contracts`)
  } else {
    mkdirSync(fileURLToPath(new URL('../src/api/contracts/', import.meta.url)), { recursive: true })
    writeFileSync(destination, output)
  }
}
