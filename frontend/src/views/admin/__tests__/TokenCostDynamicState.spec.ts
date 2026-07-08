import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function readFrontendFile(path: string) {
  return readFileSync(resolve(process.cwd(), path), 'utf8')
}

describe('TokenCost dynamic state contract', () => {
  it('loads the admin tool through typed service injection without data localStorage', () => {
    const generated = readFrontendFile('src/views/admin/tools/tokenCostLegacy.generated.ts')

    expect(generated).toContain('scope.services?.tokenCost')
    expect(generated).toContain('tokenCostService.loadState()')
    expect(generated).toContain('tokenCostService.saveState')
    expect(generated).not.toMatch(/STORAGE_KEY|localStorage|requestJson|refreshAuthToken|AUTH_REFRESH_PATH|fetch\(/)
  })

  it('keeps the public page dynamic and avoids cached calculator state', () => {
    const html = readFrontendFile('public/token-cost/index.html')

    expect(html).toContain('TOKEN_COST_STATE_PATH')
    expect(html).toContain('let state = createEmptyState();')
    expect(html).toContain('loadStateFromServer(false)')
    expect(html).not.toMatch(/STORAGE_KEY|defaultState|REPORT_FILE/)
    expect(html).not.toMatch(/localStorage\.(getItem|setItem)\(STORAGE_KEY/)
  })
})
