import type { InjectionKey, MaybeRefOrGetter } from 'vue'
import type { Runtime } from './runtime'

export const runtimeKey: InjectionKey<MaybeRefOrGetter<Runtime>> = Symbol('gcoj-runtime')
