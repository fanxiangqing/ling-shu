<script setup lang="ts">
const gradientId = `brand-${Math.random().toString(36).slice(2)}`
</script>

<template>
  <svg class="brand-mark-svg" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="Ling-Shu">
    <defs>
      <linearGradient :id="`${gradientId}-bg`" x1="10" y1="8" x2="54" y2="58" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#20372f" />
        <stop offset=".58" stop-color="#17211f" />
        <stop offset="1" stop-color="#0d1715" />
      </linearGradient>
      <linearGradient :id="`${gradientId}-mint`" x1="14" y1="12" x2="50" y2="52" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#8eeacb" />
        <stop offset="1" stop-color="#0f8f6b" />
      </linearGradient>
    </defs>

    <rect width="64" height="64" rx="14" :fill="`url(#${gradientId}-bg)`" />
    <path d="M47 6c7 4 11 11 11 20v12c0 11-9 20-20 20H26c12-3 22-12 25-24 3-11 1-20-4-28Z" fill="#0f8f6b" opacity=".22" />
    <path d="M20 17v30h18" fill="none" stroke="#ffffff" stroke-width="7" stroke-linecap="round" stroke-linejoin="round" />
    <path
      d="M43 19c-4-3-15-3-15 5 0 9 17 5 17 16 0 8-11 10-18 5"
      fill="none"
      :stroke="`url(#${gradientId}-mint)`"
      stroke-width="7"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <circle cx="47" cy="47" r="5" fill="#7be0bd" />
  </svg>
</template>
