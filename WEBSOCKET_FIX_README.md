# WebSocket Reconnection Fix

## Problem

The cattle-cluster-agent was experiencing repository sync issues where repositories would get stuck in "in progress" status after WebSocket reconnections. This happened because:

1. WebSocket connection drops
2. Connection retry loop attempts to reconnect every 5 seconds
3. When reconnection succeeds, `onConnect()` callback tries to start embedded Rancher server again
4. Multiple `rancher.Run()` calls cause controller conflicts and resource leaks
5. Repository sync stops working

## Solution

Added a simple `rancherStarted` boolean flag to prevent duplicate embedded Rancher server startups during WebSocket reconnections.

## Code Changes

### main.go
- Added `rancherStarted` global variable to track embedded server state
- Modified `onConnect()` to check flag before calling `rancher.Run()`
- Added better logging for connection events
- Added `rancherRunFunc` variable for testing

### main_test.go
- Created test to verify duplicate startup prevention
- Tests that first connection starts Rancher
- Tests that subsequent connections don't restart Rancher

## Behavior Changes

**Before:**
- WebSocket reconnection → `rancher.Run()` called multiple times → Controller conflicts → Repository sync fails

**After:**
- WebSocket reconnection → Skip `rancher.Run()` if already started → Controllers remain stable → Repository sync continues working

## Testing

Run the test:
```bash
cd cmd/agent
go test -v
```

## Validation

After applying this fix:
1. Repository refresh clicks should process quickly instead of staying "in progress"
2. WebSocket reconnections should show these log messages:
   - `"WebSocket connection established"`
   - `"WebSocket reconnected - embedded Rancher server already running"`
3. No more controller startup conflicts or resource leaks

## Impact

- **Minimal code changes** - Only added one boolean flag and enhanced logging
- **Backward compatible** - No breaking changes 
- **Low risk** - Simple logic with clear fallback behavior
- **Addresses root cause** - Prevents the duplicate startup issue entirely