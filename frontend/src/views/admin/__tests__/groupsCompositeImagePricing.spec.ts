import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const groupsViewSource = readFileSync(
  resolve(process.cwd(), 'src/views/admin/GroupsView.vue'),
  'utf8',
)

describe('Composite group image pricing controls', () => {
  it('shows image generation settings in both create and edit forms', () => {
    expect(groupsViewSource).toContain("createForm.platform === 'composite'")
    expect(groupsViewSource).toContain("editForm.platform === 'composite'")
    expect(groupsViewSource).toContain('v-model="createForm.allow_image_generation"')
    expect(groupsViewSource).toContain('v-model="editForm.allow_image_generation"')
  })
})
