import { spawn, spawnSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const dataDirectory = mkdtempSync(join(tmpdir(), 'imagesilo-e2e-'))
const binary = resolve(process.cwd(), '../bin/imagesilo')
const environment = {
  ...process.env,
  IMAGESILO_COOKIE_SECURE: 'false',
  IMAGESILO_DATA_DIR: dataDirectory,
  IMAGESILO_LISTEN_ADDRESS: '127.0.0.1:18765',
  IMAGESILO_MIGRATION_MUTATIONS: 'true',
  IMAGESILO_PROCESSING_CONCURRENCY: '1',
}

const admin = spawnSync(binary, ['admin', 'create', '--email', 'e2e@example.com', '--password-stdin'], {
  encoding: 'utf8',
  env: environment,
  input: 'imagesilo-e2e-password\n',
})
if (admin.status !== 0) {
  rmSync(dataDirectory, { recursive: true, force: true })
  process.stderr.write(admin.stderr)
  process.exit(admin.status ?? 1)
}

const migrationDirectory = join(dataDirectory, 'migrations', 'i', '2026', '08')
mkdirSync(migrationDirectory, { recursive: true })
writeFileSync(join(migrationDirectory, 'migration-e2e.webp'), Buffer.from('UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA', 'base64'))
writeFileSync(join(migrationDirectory, 'migration-mobile.webp'), Buffer.from('UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA', 'base64'))

const server = spawn(binary, ['serve'], { env: environment, stdio: 'inherit' })
let stopping = false

function stop(signal) {
  if (stopping) return
  stopping = true
  server.kill(signal)
}

process.on('SIGINT', () => stop('SIGINT'))
process.on('SIGTERM', () => stop('SIGTERM'))
server.on('exit', (code) => {
  rmSync(dataDirectory, { recursive: true, force: true })
  process.exit(code ?? 0)
})
