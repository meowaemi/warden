<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import ProblemCard from './components/ProblemCard.vue'
import { fetchSites, fetchCommands, fetchHistory, fetchDemoLink, setDemoLink, createCommand } from './lib/api.js'
import { formatTime, severityRank } from './lib/format.js'

const REFRESH_MS = 5000
const SHOW_SYSTEM_KEY = 'warden.showSystem'
const isSystem = (p) => p.target.namespace.startsWith('kube-')
function readSetting(key) {
  try {
    return localStorage.getItem(key) === 'true'
  } catch {
    return false
  }
}
function saveSetting(key, value) {
  try {
    localStorage.setItem(key, String(value))
  } catch {
  }
}

const error = ref('')
const loaded = ref(false)
const sites = ref([])
const siteID = ref('')
const activeToken = ref('')
const token = ref('') // Intentionally memory-only; never stored in browser storage.
const site = computed(() => sites.value.find(s => s.id === siteID.value))
const problems = computed(() => site.value?.messages ?? [])
const commands = ref([])
const demoLink = ref(null)
const demoBusy = ref(false)
const demoError = ref('')
async function loadDemo() {
  const currentToken = activeToken.value
  try {
    const result = await fetchDemoLink(currentToken)
    if (currentToken === activeToken.value) demoLink.value = result.simulated ? result : null
  } catch { if (currentToken === activeToken.value) demoLink.value = null }
}
async function toggleDemoLink() {
  demoBusy.value = true
  demoError.value = ''
  try {
    demoLink.value = await setDemoLink(activeToken.value, !demoLink.value.connected)
    await load()
  } catch (e) { demoError.value = e.message }
  finally { demoBusy.value = false }
}
const history = ref([])
const historyError = ref('')
const historyLoading = ref(false)
const historyLoaded = ref(false)
const commandsLoading = ref(false)
const commandError = ref('')
const commandLoadError = ref('')
const commandNotice = ref('')
const submitting = ref(false)
const pending = ref(null)
const namespace = ref('')
const deployment = ref('')
const targetUID = ref('')
const ttl = ref(10)
function fillDemoTarget() {
  if (!demoLink.value?.simulated || siteID.value !== 'demo-site' || pending.value || submitting.value) return
  namespace.value = 'demo'
  deployment.value = 'demo-app'
  targetUID.value = 'demo-deployment-uid'
}
let generation = 0
let loadGeneration = -1
let commandSequence = 0
const timeLabel = value => value && !value.startsWith('0001-') ? formatTime(value) : 'Never'
const hasObservation = computed(() => site.value?.last_success && !site.value.last_success.startsWith('0001-'))
const observationUnknown = computed(() => !!error.value || !hasObservation.value || site.value?.stale)
const statusLabels = { queued: 'Waiting for site', dispatched: 'Sent to site', applied: 'Change accepted by Kubernetes', failed: 'Failed', expired: 'Expired before execution', unknown: 'Outcome unknown — investigate before retrying' }
watch(siteID, () => { namespaceFilter.value = 'ALL'; commands.value = []; history.value = []; historyLoaded.value = false; historyError.value = ''; commandError.value = ''; commandLoadError.value = ''; commandNotice.value = ''; loadCommands() })
async function loadCommands() {
  const selected = siteID.value
  const currentToken = activeToken.value
  if (!selected || !currentToken) return
  const sequence = ++commandSequence
  commandsLoading.value = true
  try {
    const result = await fetchCommands(currentToken, selected)
    if (selected === siteID.value && currentToken === activeToken.value && sequence === commandSequence) { commands.value = result; commandLoadError.value = '' }
  } catch (e) {
    if (selected === siteID.value && currentToken === activeToken.value && sequence === commandSequence) commandLoadError.value = e.message
  } finally {
    if (selected === siteID.value && currentToken === activeToken.value && sequence === commandSequence) commandsLoading.value = false
  }
}
async function loadHistory() {
  const selected = siteID.value
  const currentToken = activeToken.value
  historyLoading.value = true
  historyError.value = ''
  try {
    const result = await fetchHistory(currentToken, selected)
    if (selected === siteID.value && currentToken === activeToken.value) { history.value = result; historyLoaded.value = true }
  } catch (e) {
    if (selected === siteID.value && currentToken === activeToken.value) historyError.value = e.message
  } finally { historyLoading.value = false }
}
async function submitCommand() {
  if (!pending.value) {
    if (!siteID.value || !namespace.value.trim() || !deployment.value.trim() || !targetUID.value.trim() || !Number.isFinite(Number(ttl.value)) || Number(ttl.value) < 1 || Number(ttl.value) > 1440) {
      commandError.value = 'Choose a site, fill all target fields and set a lifetime of 1–1440 minutes.'
      return
    }
    pending.value = { id: crypto.randomUUID(), site_id: siteID.value, kind: 'restart_deployment', namespace: namespace.value.trim(), name: deployment.value.trim(), target_uid: targetUID.value.trim(), expires_at: new Date(Date.now() + Number(ttl.value) * 60000).toISOString() }
  }
  submitting.value = true
  commandError.value = ''
  commandNotice.value = ''
  try {
    const result = await createCommand(activeToken.value, pending.value)
    commandNotice.value = `Request ${result.id} recorded: ${statusLabels[result.status] ?? result.status}.`
    pending.value = null
    await loadCommands()
  } catch (e) {
    commandError.value = e.message
    if ([400, 401, 403, 404, 409, 422].includes(e.status)) pending.value = null
  } finally { submitting.value = false }
}
function connect() {
  activeToken.value = token.value
  demoLink.value = null
  loadDemo()
  generation++
  error.value = ''
  commandError.value = ''
  commandNotice.value = ''
  history.value = []
  historyLoaded.value = false
  sites.value = []
  commands.value = []
  loaded.value = false
  load()
}
const now = ref(Date.now())

const severityFilter = ref('ALL')
const namespaceFilter = ref('ALL')
const search = ref('')
const groupBy = ref('severity')
const showSystem = ref(readSetting(SHOW_SYSTEM_KEY))

watch(showSystem, (on) => {
  saveSetting(SHOW_SYSTEM_KEY, on)
  if (!on && namespaceFilter.value.startsWith('kube-')) namespaceFilter.value = 'ALL'
})

async function load() {
  if (!activeToken.value || loadGeneration === generation) return
  const current = generation
  loadGeneration = current
  const currentToken = activeToken.value
  try {
    const result = await fetchSites(currentToken)
    if (current !== generation || currentToken !== activeToken.value) return
    sites.value = result
    if (!result.some(s => s.id === siteID.value)) siteID.value = result[0]?.id ?? ''
    error.value = ''
    await loadCommands()
    if (demoLink.value) await loadDemo()
  } catch (e) {
    if (current === generation && currentToken === activeToken.value) error.value = e.message
  } finally {
    if (loadGeneration === current) loadGeneration = -1
    if (current === generation && currentToken === activeToken.value) loaded.value = true
  }
}

let refreshTimer
let clockTimer
onMounted(() => {
  load()
  refreshTimer = setInterval(load, REFRESH_MS)
  clockTimer = setInterval(() => (now.value = Date.now()), 1000)
})
onUnmounted(() => {
  clearInterval(refreshTimer)
  clearInterval(clockTimer)
})
const shown = computed(() => problems.value.filter((p) => showSystem.value || !isSystem(p)))
const systemCount = computed(() => problems.value.filter(isSystem).length)

const counts = computed(() => {
  const c = { ERROR: 0, WARNING: 0, INFO: 0 }
  for (const p of shown.value) c[p.severity] = (c[p.severity] ?? 0) + 1
  return c
})

const namespaces = computed(() =>
  [...new Set(shown.value.map((p) => p.target.namespace))].sort(),
)

const visible = computed(() => {
  const q = search.value.trim().toLowerCase()
  return shown.value
    .filter((p) => severityFilter.value === 'ALL' || p.severity === severityFilter.value)
    .filter((p) => namespaceFilter.value === 'ALL' || p.target.namespace === namespaceFilter.value)
    .filter(
      (p) =>
        !q ||
        [p.title, p.reason, p.target.namespace, p.target.pod, p.target.container]
          .join(' ')
          .toLowerCase()
          .includes(q),
    )
    .sort(
      (a, b) =>
        (severityRank[a.severity] ?? 9) - (severityRank[b.severity] ?? 9) ||
        new Date(a.time) - new Date(b.time),
    )
})

const groups = computed(() => {
  const keyOf = {
    severity: (p) => p.severity,
    reason: (p) => p.reason,
    namespace: (p) => p.target.namespace,
  }[groupBy.value]

  const map = new Map()
  for (const p of visible.value) {
    const k = keyOf(p)
    if (!map.has(k)) map.set(k, [])
    map.get(k).push(p)
  }
  return [...map].map(([name, items]) => ({ name, items }))
})
</script>

<template>
  <div class="page">
    <header class="top">
      <div class="brand">
        <h1>Warden</h1>
        <span class="status" :class="{ bad: error }">
          {{ error ? 'Center unavailable — displayed data may be outdated' : !activeToken ? 'Connect to the control center' : !loaded ? 'Loading…' : 'Remote sites' }}
        </span>
      </div>

      <div class="summary">
        <button
          v-for="sev in ['ALL', 'ERROR', 'WARNING', 'INFO']"
          :key="sev"
          class="chip"
          :class="[sev.toLowerCase(), { active: severityFilter === sev }]"
          @click="severityFilter = sev"
        >
          {{ sev === 'ALL' ? 'All' : sev }}
          <b>{{ sev === 'ALL' ? shown.length : counts[sev] }}</b>
        </button>
      </div>
    </header>

    <form class="controls panel" @submit.prevent="connect">
      <label>Operator token <input v-model="token" type="password" autocomplete="off" placeholder="Access token" required /></label>
      <button type="submit">Connect</button>
      <span>Token stays in memory until this page closes.</span>
    </form>
    <section v-if="demoLink" class="panel demo-panel">
      <h2>SIMULATED DEMO — no real Kubernetes cluster</h2>
      <p>This control simulates the connection between the remote agent and the center. Your browser remains connected to Warden. Demo command results are simulated.</p>
      <p>Agent ↔ center: <strong>{{ demoLink.connected ? 'Connected' : 'Disconnected' }}</strong></p>
      <button :disabled="demoBusy" @click="toggleDemoLink">{{ demoBusy ? 'Updating…' : demoLink.connected ? 'Disconnect simulated site' : 'Reconnect simulated site' }}</button>
      <p v-if="demoError" role="alert">{{ demoError }}</p>
    </section>
    <section v-if="loaded && sites.length" class="panel">
      <label>Site <select v-model="siteID"><option v-for="s in sites" :key="s.id" :value="s.id">{{ s.id }}</option></select></label>
      <p>Last contact with site: <strong>{{ timeLabel(site?.last_contact) }}</strong></p>
      <p>Last successful Kubernetes observation: <strong>{{ timeLabel(site?.last_success) }}</strong></p>
      <p v-if="observationUnknown" class="warning-text">Current application state is unknown. Displayed problems are the last known observations.</p>
      <p v-if="site?.last_error">Collection notice: {{ site.last_error }}</p>
    </section>
    <p v-if="loaded && !error && !sites.length" class="empty">No sites have reported yet.</p>

    <section class="controls">
      <input v-model="search" type="search" aria-label="Search problems by pod, reason or namespace" placeholder="Search: pod, reason, namespace…" />
      <label>
        Namespace
        <select v-model="namespaceFilter">
          <option value="ALL">all</option>
          <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
        </select>
      </label>
      <label>
        Group by
        <select v-model="groupBy">
          <option value="severity">severity</option>
          <option value="reason">reason</option>
          <option value="namespace">namespace</option>
        </select>
      </label>
      <label class="toggle">
        <input v-model="showSystem" type="checkbox" />
        Show system <span>({{ systemCount }})</span>
      </label>
    </section>

    <p v-if="error" class="banner">
      {{ error }}. Check your connection and access token.
    </p>

    <p v-else-if="site && !observationUnknown && problems.length === 0" class="empty">No monitored problems were found in the latest successful observation.</p>

    <p v-else-if="site && problems.length > 0 && shown.length === 0" class="empty">
      No problems outside system namespaces.
      <button class="link" @click="showSystem = true">Show {{ systemCount }} system</button>
    </p>

    <p v-else-if="site && shown.length > 0 && visible.length === 0" class="empty">Nothing matches the filters.</p>

    <section v-for="g in groups" :key="g.name" class="group">
      <h2>{{ g.name }} <span>{{ g.items.length }}</span></h2>
      <div class="list">
        <ProblemCard v-for="p in g.items" :key="p.key" :problem="p" :now="now" />
      </div>
    </section>
    <section v-if="site" class="panel">
      <h2>Deferred restart</h2>
      <p>Request a Deployment restart when this site connects. Kubernetes acceptance does not confirm application recovery.</p>
      <p v-if="demoLink?.simulated && siteID === 'demo-site'">
        Simulated target: demo/demo-app.
        <button type="button" :disabled="submitting || !!pending" @click="fillDemoTarget">Fill simulated target</button>
      </p>
      <form class="command-form" @submit.prevent="submitCommand">
        <fieldset :disabled="submitting || !!pending">
          <label>Namespace <input v-model="namespace" required /></label>
          <label>Deployment name <input v-model="deployment" required /></label>
          <label>Deployment UID <input v-model="targetUID" required placeholder="Exact UID from Kubernetes" /></label>
          <label>Lifetime (minutes) <input v-model.number="ttl" type="number" min="1" max="1440" required /></label>
        </fieldset>
        <p v-if="pending">Confirmation was not received for request {{ pending.id }} (site {{ pending.site_id }}). Retry sends the same request, without creating a second one.</p>
        <button :disabled="submitting" type="submit">{{ submitting ? 'Submitting…' : pending ? 'Retry same request' : 'Queue restart request' }}</button>
      </form>
      <p v-if="commandError" class="banner" role="alert">{{ commandError }}</p>
      <p v-if="commandNotice" role="status">{{ commandNotice }}</p>
      <h2>Requests</h2>
      <p v-if="commandLoadError" class="banner" role="alert">{{ commandLoadError }}. Displayed request statuses may be outdated.</p>
      <p v-if="commandsLoading && !commands.length">Loading requests…</p>
      <p v-else-if="!commandLoadError && !commands.length">No requests for this site.</p>
      <article v-for="command in commands" :key="command.id" class="request">
        <strong>{{ command.namespace }}/{{ command.name }}</strong>
        <span>{{ statusLabels[command.status] ?? command.status }}</span>
        <small>{{ command.id }} · expires {{ timeLabel(command.expires_at) }}</small>
        <p v-if="command.note">{{ command.note }}</p>
      </article>
    </section>
    <section v-if="site" class="panel">
      <h2>Observation history</h2>
      <p>Recent observations retained by the center. Gaps between observations do not prove that no incidents occurred.</p>
      <button :disabled="historyLoading" @click="loadHistory">{{ historyLoading ? 'Loading…' : 'Load recent history' }}</button>
      <p v-if="historyError" class="banner" role="alert">{{ historyError }}</p>
      <p v-if="historyLoaded && !historyLoading && !historyError && !history.length">No retained observations for this site.</p>
      <article v-for="snapshot in history" :key="snapshot.id" class="request">
        <strong>{{ timeLabel(snapshot.observed_at) }}</strong>
        <span>{{ snapshot.collected ? `${(snapshot.messages ?? []).length} monitored problems observed` : 'Collection failed — application state unknown' }}</span>
        <span v-if="snapshot.error">{{ snapshot.error }}</span>
      </article>
    </section>

  </div>
</template>

<style scoped>
.page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px 16px 48px;
  display: grid;
  gap: 20px;
}
.top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.brand {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;
}
h1 {
  margin: 0;
  font-size: 24px;
  color: var(--text-h);
}
.status {
  font-size: 13px;
  color: var(--text-dim);
}
.status.bad { color: var(--error); }

.summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.chip {
  --sev: var(--text-dim);
  display: inline-flex;
  gap: 8px;
  align-items: center;
  font: inherit;
  font-size: 13px;
  color: var(--text-h);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 5px 12px;
  cursor: pointer;
}
.chip.error { --sev: var(--error); }
.chip.warning { --sev: var(--warning); }
.chip.info { --sev: var(--info); }
.chip b { color: var(--sev); }
.chip.active {
  border-color: var(--sev);
  box-shadow: inset 0 0 0 1px var(--sev);
}

.controls {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  align-items: center;
  font-size: 13px;
  color: var(--text-dim);
}
.controls input[type='search'] {
  flex: 1;
  min-width: 200px;
}
.toggle {
  cursor: pointer;
  user-select: none;
}
.toggle input {
  margin: 0;
  accent-color: var(--info);
}
.toggle span {
  color: var(--text-dim);
}
.link {
  font: inherit;
  color: var(--info);
  background: none;
  border: 0;
  padding: 0;
  cursor: pointer;
  text-decoration: underline;
}
.controls label {
  display: inline-flex;
  gap: 6px;
  align-items: center;
}
input,
select {
  font: inherit;
  font-size: 14px;
  color: var(--text-h);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 10px;
}

.banner {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--error);
  border-radius: 8px;
  color: var(--text-h);
}
.empty {
  margin: 0;
  padding: 40px 0;
  text-align: center;
  color: var(--text-dim);
}

.group {
  display: grid;
  gap: 10px;
}
.group h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-h);
  text-transform: none;
}
.group h2 span {
  color: var(--text-dim);
  font-weight: 400;
}
.list {
  display: grid;
  gap: 10px;
}
code { font-family: var(--mono); }
.panel { border: 1px solid var(--border); border-radius: 8px; padding: 16px; background: var(--surface); }
.demo-panel { border-color: var(--warning); }
.panel h2 { font-size: 17px; margin: 0 0 10px; color: var(--text-h); }
.panel p { margin: 8px 0; }
.warning-text { color: var(--warning); }
button { font: inherit; padding: 6px 12px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-h); cursor: pointer; }
button:disabled { opacity: .6; cursor: wait; }
.command-form { display: grid; gap: 12px; margin: 12px 0; }
.command-form fieldset { border: 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; }
.command-form label { display: grid; gap: 4px; }
.command-form button { justify-self: start; }
.request { display: grid; gap: 4px; padding: 12px 0; border-top: 1px solid var(--border); overflow-wrap: anywhere; }
.request small { color: var(--text-dim); }
</style>
