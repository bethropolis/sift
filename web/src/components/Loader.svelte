<script lang="ts">
  interface Props {
    label?: string;
    size?: number;
  }

  let { label = 'Loading...', size = 40 }: Props = $props();
</script>

<div class="loader" role="status" aria-label={label}>
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    <rect width="24" height="24" rx="5" fill="var(--logo-bg)" stroke="var(--logo-border)" stroke-width="1" />
    <rect class="bar bar-1" x="4.5" y="8" width="11" height="2.5" rx="1.25" fill="var(--accent)" />
    <rect class="bar bar-2" x="8.5" y="13.5" width="11" height="2.5" rx="1.25" fill="var(--accent)" />
  </svg>
  <span class="loader-label">{label}</span>
</div>

<style>
  .loader {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    min-height: 200px;
    padding: 24px;
    color: var(--code-dim);
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .loader-label {
    opacity: 0.85;
  }
  .bar {
    animation: bar-slide 1.1s ease-in-out infinite;
  }
  .bar-2 {
    animation-delay: 0.28s;
  }
  @keyframes bar-slide {
    0%,
    100% {
      transform: translateX(0);
      opacity: 0.55;
    }
    50% {
      transform: translateX(4px);
      opacity: 1;
    }
  }
</style>
