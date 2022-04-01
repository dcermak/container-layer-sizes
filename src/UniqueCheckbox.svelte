<script lang="ts">
  export let group: any[] | undefined = undefined;
  export let value: any | undefined = undefined;
  export let label: string | undefined = undefined;
</script>

<label>
  <input type="checkbox" bind:group {value} class="unique-class-name" />
  {label}
  <slot />
</label>
