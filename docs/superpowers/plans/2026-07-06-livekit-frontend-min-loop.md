# LiveKit Frontend Minimum Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect the existing Vue room page to LiveKit so two browser tabs can join the same room, publish local camera/microphone, and render local and remote media.

**Architecture:** Keep Go WebSocket chat unchanged. Add a small LiveKit client wrapper and lightweight video components, then let `RoomView.vue` orchestrate the REST join, LiveKit connect, and app WebSocket connect sequence.

**Tech Stack:** Vue 3, TypeScript, Pinia, Element Plus, livekit-client, Vite.

---

### Task 1: Dependencies and Test Harness

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`
- Create: `frontend/src/livekit/connection.test.ts`
- Create: `frontend/src/livekit/connection.ts`

- [ ] Install `livekit-client` for runtime use and `vitest` for small frontend unit tests.
- [ ] Write a failing test for LiveKit status mapping before production implementation.
- [ ] Implement the minimal status mapping helper.
- [ ] Run the focused unit test.

### Task 2: LiveKit Client Wrapper

**Files:**
- Modify: `frontend/src/livekit/connection.ts`
- Test: `frontend/src/livekit/connection.test.ts`

- [ ] Add tests for local and remote track view state updates.
- [ ] Implement a wrapper class that creates a LiveKit `Room`, connects with `livekitUrl + livekitToken`, enables camera/microphone, listens to room events, and disconnects cleanly.
- [ ] Keep wrapper output as plain view models so Vue components do not depend on LiveKit internals more than necessary.

### Task 3: Video Components

**Files:**
- Create: `frontend/src/components/VideoTile.vue`
- Create: `frontend/src/components/VideoGrid.vue`
- Modify: `frontend/src/types.ts`

- [ ] Add typed video tile view models.
- [ ] Create `VideoTile` that attaches/detaches LiveKit audio/video tracks.
- [ ] Create `VideoGrid` that renders local and remote tiles with stable layout and empty states.

### Task 4: Room Page Integration

**Files:**
- Modify: `frontend/src/views/RoomView.vue`
- Modify: `frontend/src/stores/connection.ts`
- Modify: `frontend/src/assets/main.css`

- [ ] Connect LiveKit after `joinRoom` succeeds.
- [ ] Show LiveKit and app WebSocket status independently.
- [ ] Keep Go WebSocket chat behavior unchanged.
- [ ] Disconnect LiveKit and app WebSocket on leave or route unmount.
- [ ] Replace placeholder media panel with `VideoGrid`.

### Task 5: Verification

**Commands:**
- `npm run build`
- `npx vitest run src/livekit/connection.test.ts`

**Manual checks for the user:**
- Keep `docker compose up -d redis livekit` running.
- Keep Go room service running on `:8080`.
- Start frontend with `npm run dev`.
- Open two browser tabs with different users and verify they can join the same room and see/hear each other.
