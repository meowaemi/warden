<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import ProblemCard from './components/ProblemCard.vue'
import { fetchMessages } from './lib/api.js'
import { formatTime, severityRank } from './lib/format.js'

const REFRESH_MS = 5000
const SHOW_SYSTEM_KEY = 'warden.showSystem'

// System namespaces belong to Kubernetes itself and to the local cluster
// tooling (kube-system, kube-public, kube-node-lease, ...).
const isSystem = (p) => p.target.namespace.startsWith('kube-')

// readSetting and saveSetting keep a per-browser preference; storage may be
// unavailable (private mode), and the page must work without it.
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
    // ignore
  }
}

const problems = ref([])
const error = ref('')
const loaded = ref(false)
const updatedAt = ref('')
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
  try {
    problems.value = await fetchMessages()
    error.value = ''
    updatedAt.value = new Date().toISOString()
  } catch (e) {
    error.value = e.message
  } finally {
    loaded.value = true
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

// shown is every problem the page may display: system ones only on request.
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
          {{ error ? 'warden is unreachable' : updatedAt ? `updated ${formatTime(updatedAt)}` : 'loading…' }}
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

    <section class="controls">
      <input v-model="search" type="search" placeholder="Search: pod, reason, namespace…" />
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
      {{ error }}. Make sure warden is running (<code>go run .</code>) and listening on :8080.
    </p>

    <p v-else-if="loaded && problems.length === 0" class="empty">No problems. The cluster is healthy.</p>

    <p v-else-if="loaded && shown.length === 0" class="empty">
      No problems outside system namespaces.
      <button class="link" @click="showSystem = true">Show {{ systemCount }} system</button>
    </p>

    <p v-else-if="loaded && visible.length === 0" class="empty">Nothing matches the filters.</p>

    <section v-for="g in groups" :key="g.name" class="group">
      <h2>{{ g.name }} <span>{{ g.items.length }}</span></h2>
      <div class="list">
        <ProblemCard v-for="p in g.items" :key="p.key" :problem="p" :now="now" />
      </div>
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
</style>
