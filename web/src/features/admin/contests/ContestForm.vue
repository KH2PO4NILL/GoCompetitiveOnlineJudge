<script setup lang="ts">
import { reactive } from 'vue'
import type { StringRecord } from '../../../lib/forms'
import { contestModes, registrationPolicies, visibilities } from '../../../lib/labels'
import TextField from '../../../components/ui/TextField.vue'

/**
 * 比赛编辑表单（创建 / 工作台复用）。
 * 时间字段以 datetime-local 字符串保存，交由 lib/forms contestValues 与 lib/freeze freezePolicy 转换校验。
 */
const defaults: StringRecord = {
  title: '',
  description: '',
  mode: 'icpc',
  start_at: '',
  end_at: '',
  registration_start_at: '',
  registration_end_at: '',
  registration_policy: 'open',
  visibility: 'public',
  scoreboard_visibility: 'public',
  invite_code: '',
  freeze_policy: 'none',
  freeze_at: '',
  freeze_seconds: '',
}

const values = reactive<StringRecord>({ ...defaults })

function load(next: StringRecord) {
  Object.assign(values, defaults, next)
}

function snapshot(): StringRecord {
  return { ...values }
}

defineExpose({ load, snapshot })
</script>

<template>
  <div class="stack">
    <TextField v-model="values.title" label="比赛标题" name="title" required placeholder="如 2026 春季公开赛" />
    <TextField
      v-model="values.description" label="比赛描述" name="description" type="textarea" :rows="6"
      hint="支持 Markdown（含 LaTeX 公式）。"
    />

    <div class="grid-two">
      <div class="field">
        <label for="cf-mode">赛制</label>
        <select id="cf-mode" v-model="values.mode" name="mode">
          <option v-for="(label, value) in contestModes" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="cf-registration-policy">报名策略</label>
        <select id="cf-registration-policy" v-model="values.registration_policy" name="registration_policy">
          <option v-for="(label, value) in registrationPolicies" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="cf-visibility">可见性</label>
        <select id="cf-visibility" v-model="values.visibility" name="visibility">
          <option v-for="(label, value) in visibilities" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="cf-scoreboard-visibility">榜单可见性</label>
        <select id="cf-scoreboard-visibility" v-model="values.scoreboard_visibility" name="scoreboard_visibility">
          <option v-for="(label, value) in visibilities" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
    </div>

    <div class="grid-two">
      <TextField v-model="values.start_at" label="开始时间" name="start_at" type="datetime-local" required />
      <TextField v-model="values.end_at" label="结束时间" name="end_at" type="datetime-local" required />
      <TextField v-model="values.registration_start_at" label="报名开始（选填）" name="registration_start_at" type="datetime-local" />
      <TextField v-model="values.registration_end_at" label="报名截止（选填）" name="registration_end_at" type="datetime-local" />
    </div>

    <TextField
      v-model="values.invite_code" label="邀请码" name="invite_code"
      :required="values.registration_policy === 'invite_only'"
      :hint="values.registration_policy === 'invite_only' ? '邀请码报名模式下必填。' : '仅邀请码报名模式需要。'"
    />

    <section class="freeze">
      <h3>自动封榜</h3>
      <p class="small muted">自动封榜仅适用于 ICPC 赛制。</p>
      <div class="field">
        <label for="cf-freeze-policy">封榜方式</label>
        <select id="cf-freeze-policy" v-model="values.freeze_policy" name="freeze_policy">
          <option value="none">不自动封榜</option>
          <option value="at" :disabled="values.mode !== 'icpc'">指定时间封榜</option>
          <option value="before_end" :disabled="values.mode !== 'icpc'">赛前 N 秒封榜</option>
        </select>
      </div>
      <TextField
        v-if="values.freeze_policy === 'at'"
        v-model="values.freeze_at" label="封榜时间" name="freeze_at" type="datetime-local"
        hint="须晚于开始且早于结束。"
      />
      <TextField
        v-if="values.freeze_policy === 'before_end'"
        v-model="values.freeze_seconds" label="赛前封榜秒数" name="freeze_seconds" type="number"
        hint="正整数，且须小于比赛时长。"
      />
    </section>
  </div>
</template>

<style scoped>
.grid-two { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0 var(--space-4); }
.freeze { display: grid; gap: var(--space-2); border-top: 1px solid var(--border); padding-top: var(--space-3); }
.freeze h3 { margin: 0; }
</style>
