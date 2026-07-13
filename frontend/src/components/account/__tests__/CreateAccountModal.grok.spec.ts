import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)
const credentialsFieldsSource = readFileSync(
  resolve(process.cwd(), 'src/components/account/AccountAPIKeyCredentialsFields.vue'),
  'utf8'
)

describe('CreateAccountModal Grok account types', () => {
  it('offers API-key setup alongside OAuth with the official xAI default', () => {
    expect(source).toContain(':options="grokAccountTypeOptions"')
    expect(source).toContain('const grokAccountTypeOptions')
    expect(source).toContain("value: 'apikey'")
    expect(source).toContain("newPlatform === 'grok'")
    expect(source).toContain("? 'https://api.x.ai/v1'")
    expect(credentialsFieldsSource).toContain("props.platform === 'grok'")
    expect(credentialsFieldsSource).toContain("return 'xai-...'")
  })

  it('defaults to a compact basic tab and keeps advanced settings behind a second tab', () => {
    expect(source).toContain('<AccountFormTabs v-model="activeFormTab"')
    expect(source).toContain("const activeFormTab = ref<AccountFormTab>('basic')")
    expect(source).toContain(':show-notes="activeFormTab === \'advanced\'"')
    expect(source).toContain(':section="activeFormTab === \'basic\' ? \'core\' : \'advanced\'"')
  })
})
