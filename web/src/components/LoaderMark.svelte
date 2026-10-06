<script lang="ts">
  interface Props {
    size?: number;
    /**
     * Controls the animation. The embedding body decides: true while busy,
     * false for a static brand mark. Defaults on so bare usages animate.
     */
    animated?: boolean;
  }

  let { size = 12, animated = true }: Props = $props();
</script>

<!--
  sift mark without the logo container: borderless bars in the theme accent,
  so it blends into any surface on any theme. Tightly cropped viewBox keeps
  button alignment stable.
-->
<svg
  width={size}
  height={(size * 10) / 17}
  viewBox="3.5 7 17 10"
  fill="none"
  xmlns="http://www.w3.org/2000/svg"
  aria-hidden="true"
  class="loader-mark"
  class:static={!animated}
>
  <rect class="bar bar-1" x="4.5" y="8" width="11" height="2.5" rx="1.25" fill="var(--accent)" />
  <rect class="bar bar-2" x="8.5" y="13.5" width="11" height="2.5" rx="1.25" fill="var(--accent)" />
</svg>

<style>
  .loader-mark {
    flex-shrink: 0;
    display: block;
    overflow: visible;
  }
  .bar {
    animation: bar-slide 1.2s ease-in-out infinite;
  }
  .bar-2 {
    animation-delay: 0.3s;
  }
  .static .bar {
    animation: none;
    opacity: 0.9;
  }
  @keyframes bar-slide {
    0%,
    100% {
      transform: translateX(0);
      opacity: 0.45;
    }
    50% {
      transform: translateX(3px);
      opacity: 1;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .bar {
      animation: none;
      opacity: 0.9;
    }
  }
</style>
