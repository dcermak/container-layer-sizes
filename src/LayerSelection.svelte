<script lang="ts">
  import type { DataRouteReply } from "./types";
  import { formatByte } from "./util";

  const printLayerSize = (layerDigest: string) =>
    formatByte(data[layerDigest as keyof DataRouteReply].total_size);
  const printCreatedBy = (layerDigest: string) =>
    data[layerDigest as keyof DataRouteReply].CreatedBy;

  const trimString = (s: string, trimTo: number) =>
    trimTo === -1 ? s : s.slice(0, trimTo);

  export let data: DataRouteReply | undefined = undefined;
  export let showCreatedBy: boolean = true;
  export let trimDigestTo: number = -1;
  export let digest: string;

  let layers: string[] = [];
  $: if (data !== undefined) {
    layers = Object.keys(data);
  }
</script>

<table>
  <thead>
    <tr>
      <td>layer</td>
      <td>total size</td>
      {#if showCreatedBy}
        <td>created by</td>
      {/if}
    </tr>
  </thead>
  {#each layers as layer}
    <tr>
      <td>
        <label>
          <input type="radio" bind:group={digest} value={layer} name="layer" />
          {trimString(layer, trimDigestTo)}
        </label>
      </td>
      <td>{printLayerSize(layer)}</td>
      {#if showCreatedBy}
        <td>{printCreatedBy(layer)}</td>
      {/if}
    </tr>
  {/each}
</table>
