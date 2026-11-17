<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import type { ContestSummary } from '../../api/contracts/app'
import { useReducedMotion } from '../../composables/useReducedMotion'
import { contestModes, contestStatuses, labelOf } from '../../lib/labels'
import { formatTime } from '../../lib/time'
import Badge from '../../components/ui/Badge.vue'
import { contestStatusTone } from '../contests/helpers'

const props = defineProps<{ contests: ContestSummary[] }>()

const index = ref(0)
const hovering = ref(false)
const paused = ref(false)
const reduced = useReducedMotion()

const pausedNow = computed(() => hovering.value || paused.value)
const showControls = computed(() => props.contests.length > 1)

watch(() => props.contests.length, (length) => {
  if (index.value >= length) index.value = 0
})

let timer: ReturnType<typeof setInterval> | undefined

function stop() {
  if (timer !== undefined) {
    clearInterval(timer)
    timer = undefined
  }
}

function start() {
  stop()
  if (reduced.value || pausedNow.value || props.contests.length < 2) return
  timer = setInterval(() => {
    index.value = (index.value + 1) % props.contests.length
  }, 6000)
}

watch([reduced, pausedNow, () => props.contests.length], start, { immediate: true })
onBeforeUnmount(stop)

function go(next: number) {
  const length = props.contests.length
  if (!length) return
  index.value = (next + length) % length
  start()
}

function togglePaused() {
  paused.value = !paused.value
}
</script>

<template>
  <div
    class="hero-carousel" :class="{ panel: contests.length, 'is-empty': !contests.length }"
    aria-roledescription="carousel" aria-label="最近比赛"
    @mouseenter="hovering = true" @mouseleave="hovering = false"
    @focusin="hovering = true" @focusout="hovering = false"
  >
    <template v-if="contests.length">
      <div class="carousel-viewport">
        <div class="carousel-track" :style="{ transform: `translateX(-${index * 100}%)` }">
          <RouterLink
            v-for="(contest, i) in contests" :key="String(contest.contest_id)"
            class="carousel-slide" :to="`/contests/${contest.contest_id}`"
            :tabindex="i === index ? 0 : -1" :aria-hidden="i === index ? undefined : 'true'"
          >
            <div class="slide-meta">
              <Badge :tone="contestStatusTone(contest.status)" :running="contest.status === 'running'">
                {{ labelOf(contestStatuses, contest.status) }}
              </Badge>
              <span>{{ labelOf(contestModes, contest.mode) }}</span>
              <span v-if="contest.registration_open">报名开放</span>
            </div>
            <h2 class="slide-title">{{ contest.title }}</h2>
            <p class="slide-time">{{ formatTime(contest.start_at) }} — {{ formatTime(contest.end_at) }}</p>
            <div class="slide-foot">
              <span>#{{ contest.contest_id }}</span>
              <span>{{ contest.participant_count }} 人参赛</span>
            </div>
            <span class="slide-cta">查看比赛 →</span>
          </RouterLink>
        </div>
      </div>
      <div v-if="showControls" class="carousel-bar">
        <button class="quiet sm" type="button" @click="go(index - 1)">上一场</button>
        <div class="carousel-dots">
          <button
            v-for="(contest, i) in contests" :key="String(contest.contest_id)" type="button"
            class="dot" :class="{ active: i === index }"
            :aria-label="`第 ${i + 1} 场：${contest.title}`"
            :aria-current="i === index ? 'true' : undefined"
            @click="go(i)"
          />
        </div>
        <button class="quiet sm" type="button" @click="togglePaused">{{ paused ? '继续轮播' : '暂停轮播' }}</button>
        <button class="quiet sm" type="button" @click="go(index + 1)">下一场</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.hero-carousel { min-width: 0; min-height: 280px; }
.hero-carousel.is-empty { background: var(--surface-sunken); border-radius: var(--radius); }
.hero-carousel.panel { overflow: hidden; }
.carousel-viewport { overflow: hidden; }
.carousel-track { display: flex; transition: transform 350ms ease; }
.carousel-slide {
  flex: 0 0 100%; min-width: 0; min-height: 230px;
  display: flex; flex-direction: column; justify-content: center; gap: var(--space-2);
  padding: var(--space-5) var(--space-4);
  color: inherit; text-decoration: none;
  border-bottom: 1px solid var(--border);
}
.carousel-slide:hover { background: var(--surface-sunken); }
.carousel-slide:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
.slide-meta {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
  font-size: var(--text-xs); color: var(--muted);
}
.slide-title { margin: 0; font-size: var(--text-lg); overflow-wrap: anywhere; }
.slide-time { margin: 0; font-size: var(--text-sm); color: var(--muted); }
.slide-foot { display: flex; gap: var(--space-4); font-size: var(--text-xs); color: var(--faint); }
.slide-cta { font-size: var(--text-sm); font-weight: 500; color: var(--accent); }
.carousel-bar {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
  padding: var(--space-2) var(--space-3);
}
.carousel-dots { display: flex; gap: 6px; margin: 0 auto; }
.dot {
  width: 8px; height: 8px; padding: 0;
  border: none; border-radius: 50%;
  background: var(--border-strong);
}
.dot.active { background: var(--accent); }
.dot:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
