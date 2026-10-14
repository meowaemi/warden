<script setup>
import { computed } from 'vue'
import { formatAgo, formatTime } from '../lib/format.js'

const props = defineProps({
  problem: { type: Object, required: true },
  now: { type: Number, required: true },
})

const target = computed(() => {
  const t = props.problem.target
  return t.container ? `${t.namespace}/${t.pod} · ${t.container}` : `${t.namespace}/${t.pod}`
})

// rows are the problem's details followed by the fields every problem has.
const rows = computed(() => [
  ...(props.problem.details ?? []),
  { key: 'Reason', value: props.problem.reason },
  { key: 'Rule', value: props.problem.rule },
  { key: 'Since', value: formatTime(props.problem.time) },
])
</script>

<template>
  <article class="card" :class="problem.severity.toLowerCase()">
    <header class="head">
      <span class="badge">{{ problem.severity }}</span>
      <h3 class="title">{{ problem.title }}</h3>
      <span class="since" :title="formatTime(problem.time)">{{ formatAgo(problem.time, now) }}</span>
    </header>

    <p class="target">{{ target }}</p>
    <p v-if="problem.text" class="text">{{ problem.text }}</p>

    <dl class="details">
      <template v-for="d in rows" :key="d.key">
        <dt>{{ d.key }}</dt>
        <dd>{{ d.value }}</dd>
      </template>
    </dl>
  </article>
</template>

<style scoped>
.card {
  --sev: var(--info);
  border: 1px solid var(--border);
  border-left: 4px solid var(--sev);
  border-radius: 8px;
  background: var(--surface);
  padding: 14px 16px;
  display: grid;
  gap: 8px;
  min-width: 0;
}
.card.error { --sev: var(--error); }
.card.warning { --sev: var(--warning); }

.head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.badge {
  font: 600 11px/1 var(--mono);
  letter-spacing: 0.04em;
  color: var(--sev);
  border: 1px solid var(--sev);
  border-radius: 4px;
  padding: 3px 6px;
}
.title {
  margin: 0;
  font-size: 16px;
  color: var(--text-h);
  flex: 1;
  min-width: 0;
}
.since {
  font-size: 13px;
  color: var(--text-dim);
  white-space: nowrap;
}
.target {
  margin: 0;
  font-family: var(--mono);
  font-size: 13px;
  color: var(--text-h);
  overflow-wrap: anywhere;
}
.text {
  margin: 0;
  font-size: 14px;
}
.details {
  margin: 0;
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 12px;
  font-size: 13px;
}
.details dt { color: var(--text-dim); }
.details dd {
  margin: 0;
  font-family: var(--mono);
  overflow-wrap: anywhere;
}
</style>
