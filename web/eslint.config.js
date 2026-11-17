import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'

export default tseslint.config(
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      'src/api/contracts/**',
      'playwright-report/**',
      'test-results/**',
      'coverage/**',
      'src/**/*.tsx',
      'src/**/*.tsx/**',
    ],
  },
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/essential'],
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: { parser: tseslint.parser },
    },
    rules: {
      // 单组件名（如 App.vue）属于常见惯例，不作为错误
      'vue/multi-word-component-names': 'off',
    },
  },
)
