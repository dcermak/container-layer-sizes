<script lang="ts">
  export let showCreatedBy: boolean = true;
  export let trimDigestTo: number = -1;
</script>

<details open>
  <summary>Configure the table display</summary>
  <label>
    <input type="checkbox" bind:checked={showCreatedBy} name="showCreatedBy" />
    Show Created By Column
  </label>
  <br />
  <label for="trimDigestTo">Trim the digest to: </label>
  <input type="number" bind:value={trimDigestTo} name="trimDigestTo" />
</details>
