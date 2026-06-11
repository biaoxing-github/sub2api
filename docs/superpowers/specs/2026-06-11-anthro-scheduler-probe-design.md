# Anthro Protocol Scheduler Probe Design

**Date:** 2026-06-11  
**Author:** Devil  
**Status:** Draft

## Overview

Fix three issues in the anthro protocol scheduling system:

1. **Manual test default model**: Change anthro account manual test default from current to `claude-opus-4-8`
2. **Scheduler pool state sync**: Manual probe success must update scheduling pool state visibility
3. **Error-based probe intervals**: Replace fixed 2s probe loop with error-count graduated backoff (1s/3s/10s/30s/1min/5min)
4. **Scheduling pool manual probe UI**: Add manual probe button to scheduling pool page with full state recovery

## Problem Statement

### Current Issues

1. **Default Model Mismatch**: Anthro manual test uses outdated default model instead of `claude-opus-4-8`
2. **State Sync Gap**: Manual probe success doesn't feed back into scheduler state; accounts remain blocked/degraded in pool view even after successful manual test
3. **Fixed Probe Interval**: Current exhaustion probe loops at fixed 2s regardless of error pattern, hammering upstream with bad accounts
4. **No Manual Probe in Pool**: Scheduling pool page shows blocked/degraded accounts but provides no quick recovery action

### Root Cause

**Data structure issue**: Manual probe success is not treated as input to scheduler state. The "success event" exists but doesn't become the source of truth for scheduling decisions.

**Owner ambiguity**: State recovery responsibility is split across frontend assumptions and partial backend updates. No single service layer function owns the full recovery closure.

## Design Principles

1. **Data structure first**: Fix the flow so manual probe success becomes scheduler state input
2. **Clear ownership**: Backend service layer owns account state, runtime blocks, health degradation, and error counts
3. **No state faking**: Frontend triggers and displays; backend computes and persists
4. **Minimal scope**: Only add manual probe to anthro/antigravity pool; don't expand OpenAI hot path

## Architecture

### Component Responsibilities

```
Frontend (AccountSchedulingPoolView.vue)
├─ Display manual probe button (anthro/antigravity only)
├─ Call backend manual probe API
├─ Show result message
└─ Refresh pool snapshot on success

Handler (account_handler.go)
├─ POST /admin/accounts/:id/manual-probe
├─ Call AccountTestService.TestAccountConnectionWithResult
├─ On success: call service layer recovery function
└─ Return structured result + updated account snapshot

Service Layer
├─ AccountTestService.TestAccountConnectionWithResult
│  └─ Execute real upstream probe, return structured result
└─ RecoverAccountAfterManualProbe(accountID, platform)
   ├─ Set schedulable=true, status=active (if inactive)
   ├─ Clear runtime blocks
   ├─ Clear health degradation
   ├─ Reset error count to zero
   └─ Return recovered account snapshot
```

### State Recovery Flow

```
User clicks "Manual Probe" in pool page
  ↓
POST /admin/accounts/:id/manual-probe
  ↓
TestAccountConnectionWithResult(accountID, modelID, prompt, mode)
  ├─ Execute real upstream probe
  ├─ Return AccountTestConnectionResult
  │   - Success: true/false
  │   - ErrorMessage, HTTPStatus
  │   - LatencyMs, FirstTokenMs
  └─ If Success == true:
       ↓
     RecoverAccountAfterManualProbe(accountID, platform)
       ├─ Update account table:
       │   - schedulable = true
       │   - status = "active" (if currently "error"/"inactive")
       ├─ Clear runtime blocks for anthro
       ├─ Reset derived_health to healthy
       ├─ Reset error count to zero
       └─ Return updated Account snapshot
  ↓
Handler returns JSON:
{
  "success": true,
  "result": { ... probe result ... },
  "account": { ... recovered account state ... }
}
  ↓
Frontend:
  - Show success message
  - Refresh pool (call listSchedulingPool)
  - Account row now shows pool_status="schedulable", health=healthy
```

## Detailed Design

### 1. Default Model Change

**File**: `backend/internal/service/account_test_service.go`

**Current**:
```go
testModelID := modelID
if testModelID == "" {
    testModelID = claude.DefaultTestModel
}
```

**Change to**:
```go
testModelID := modelID
if testModelID == "" {
    testModelID = "claude-opus-4-8"
}
```

**Scope**: Only affects anthro account manual test when model not specified. Does not affect:
- Bedrock/Vertex (uses same default then branches)
- API Key accounts (model mapping still applies)
- Calls that explicitly pass modelID

### 2. State Recovery Function

**New function**: `RecoverAccountAfterManualProbe(ctx context.Context, accountID int64, platform string) (*Account, error)`

**Location**: `backend/internal/service/account_service.go` or `openai_gateway_service.go`

**Responsibilities**:
1. Load account by ID
2. Validate platform matches
3. Update account fields:
   - `schedulable = true`
   - `status = "active"` (if currently `error` or `inactive`)
4. Clear runtime blocks related to platform
5. Clear/reset health degradation state
6. Reset error count to zero (for probe interval logic)
7. Persist changes
8. Return updated account

**Why service layer**: State machine logic belongs where the state lives, not in handlers or frontend.

### 3. Error-Count Probe Intervals

**Mapping**:
```go
func probeIntervalFromErrorCount(errorCount int) time.Duration {
    switch {
    case errorCount <= 1:
        return 1 * time.Second
    case errorCount == 2:
        return 3 * time.Second
    case errorCount == 3:
        return 10 * time.Second
    case errorCount == 4:
        return 30 * time.Second
    case errorCount == 5:
        return 1 * time.Minute
    default: // >= 6
        return 5 * time.Minute
    }
}
```

**Behavior**:
- **On probe success**: error count = 0, next auto-probe in 1s
- **On probe failure**: error count += 1, next auto-probe interval increases
- **Manual probe**: not subject to interval restriction; admin click = immediate execution
- **Manual success**: force clear blocks and zero error count

**Why graduated**: Bad accounts don't hammer upstream; good accounts recover quickly.

### 4. New API Endpoint

**Route**: `POST /api/v1/admin/accounts/:id/manual-probe`

**Handler**: `AccountHandler.ManualProbe`

**Request Body**:
```json
{
  "model": "claude-opus-4-8",
  "prompt": "hi",
  "mode": ""
}
```

**Response (Success)**:
```json
{
  "success": true,
  "result": {
    "success": true,
    "message": "Connection successful",
    "latency_ms": 234,
    "first_token_ms": 180,
    "http_status": 200,
    "reason": "200_ok"
  },
  "account": {
    "id": 123,
    "name": "anthro-oauth-001",
    "platform": "anthropic",
    "schedulable": true,
    "status": "active",
    "derived_health": {
      "state": "healthy",
      "label": "正常"
    }
  }
}
```

**Response (Failure)**:
```json
{
  "success": false,
  "result": {
    "success": false,
    "message": "API returned 401",
    "latency_ms": 120,
    "http_status": 401,
    "reason": "401_unauthorized",
    "error": "Invalid authentication"
  }
}
```

### 5. Frontend Changes

**File**: `frontend/src/views/admin/AccountSchedulingPoolView.vue`

**Add button in actions column**:
```vue
<button
  v-if="shouldShowManualProbe(item.account)"
  type="button"
  data-test="manual-probe"
  class="btn btn-primary px-2 py-1 text-sm ml-2"
  :disabled="isProbing(item.account.id)"
  @click="manualProbe(item)"
>
  <Icon name="refresh" size="sm" :class="isProbing(item.account.id) ? 'animate-spin' : ''" />
  <span class="ml-1">{{ t('admin.accountSchedulingPool.manualProbe') }}</span>
</button>
```

**Button visibility**:
```typescript
function shouldShowManualProbe(account: Account): boolean {
  return account.platform === 'anthropic' || 
         (account.platform === 'antigravity' && account.mixed_scheduling_enabled)
}
```

**Probe logic**:
```typescript
const probingIds = ref<Set<number>>(new Set())

async function manualProbe(item: AccountSchedulingPoolItem) {
  const accountId = item.account.id
  probingIds.value.add(accountId)
  error.value = ''
  message.value = ''

  try {
    const result = await manualProbeAccount(accountId, {
      model: 'claude-opus-4-8'
    })

    if (result.success && result.result.success) {
      message.value = t('admin.accountSchedulingPool.probeSuccess', {
        name: item.account.name,
        latency: result.result.latency_ms
      })
      await loadPool()
    } else {
      error.value = result.result.message || t('admin.accountSchedulingPool.probeFailed')
    }
  } catch (err: any) {
    error.value = err.message || t('common.error')
  } finally {
    probingIds.value.delete(accountId)
  }
}
```

**API client addition** (`frontend/src/api/admin/accounts.ts`):
```typescript
export async function manualProbeAccount(
  id: number,
  payload?: {
    model?: string
    prompt?: string
    mode?: string
  }
): Promise<{
  success: boolean
  result: {
    success: boolean
    message: string
    latency_ms?: number
    first_token_ms?: number
    http_status?: number
    reason?: string
    error?: string
  }
  account?: Account
}> {
  const { data } = await apiClient.post(
    `/admin/accounts/${id}/manual-probe`,
    payload || {}
  )
  return data
}
```

**i18n additions** (`zh.ts` / `en.ts`):
```typescript
accountSchedulingPool: {
  manualProbe: '人工探测',
  probeSuccess: '{name} 探测成功，延迟 {latency}ms',
  probeFailed: '探测失败',
}
```

## State Fields to Recover

| Field | Table/Location | Recovered Value | Note |
|-------|----------------|-----------------|------|
| `schedulable` | `account` | `true` | Account enters scheduler pool |
| `status` | `account` | `"active"` | If currently `error` or `inactive` |
| runtime block | memory/cache | cleared/expired | Anthro-related blocks |
| derived_health | computed | `healthy` | Account composite health |
| path_health error count | memory | zero | Affects next auto-probe interval |

## Testing Strategy

### Backend Tests (Go)

1. **Default model test**: Verify empty modelID defaults to `claude-opus-4-8`
2. **Recovery function test**: Create blocked account, simulate manual probe success, verify all state fields recovered
3. **Error interval mapping test**: Test error counts 0-10 map to correct intervals
4. **Handler test**: POST endpoint returns structured result + account snapshot

### Frontend Tests (Vitest)

1. **Button visibility**: Verify button shows only for anthro/antigravity accounts
2. **Success flow**: Mock success, verify message + pool refresh
3. **Failure flow**: Mock failure, verify error message + no pool refresh

## Verification Plan

**Local**:
1. `go test ./backend/internal/service/... ./backend/internal/handler/admin/...`
2. `npm run test AccountSchedulingPoolView`
3. `npm run typecheck`
4. `go build ./backend/cmd/sub2api`

**Manual smoke test**:
1. Create blocked anthro account
2. Click manual probe in pool page
3. Verify success → pool refreshes → account shows schedulable
4. Test with invalid credentials → verify failure message

## Files Changed

| File | Change |
|------|--------|
| `backend/internal/service/account_test_service.go` | Change default model to `claude-opus-4-8` |
| `backend/internal/service/account_service.go` | Add `RecoverAccountAfterManualProbe` |
| `backend/internal/service/openai_scheduler_exhaustion_probe.go` | Add error-count interval mapping |
| `backend/internal/handler/admin/account_handler.go` | Add `ManualProbe` handler |
| `frontend/src/api/admin/accounts.ts` | Add `manualProbeAccount` function |
| `frontend/src/views/admin/AccountSchedulingPoolView.vue` | Add manual probe button + logic |
| `frontend/src/i18n/locales/zh.ts` | Add i18n |
| `frontend/src/i18n/locales/en.ts` | Add i18n |
| Backend test files | Add coverage |
| Frontend test files | Add coverage |

## Success Criteria

1. Anthro manual test without model uses `claude-opus-4-8`
2. Manual probe success clears blocks and sets schedulable
3. Pool refreshes after success showing schedulable status
4. Error intervals follow 1s/3s/10s/30s/1min/5min ladder
5. Button appears only for anthro/antigravity in pool
6. All tests pass
