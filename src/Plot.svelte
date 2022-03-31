<script lang="ts">
  import { onMount } from "svelte";
  import Sunburst from "sunburst-chart";
  import type { SunburstChartInstance } from "sunburst-chart";
  import type { DataRouteReply } from "./types";
  import { dirToDataNode } from "./fs-tree";
  import type { Dir } from "./fs-tree";
  import { formatByte } from "./util";

  export let data: DataRouteReply | undefined = undefined;

  let layers: string[] = [];
  let showCreatedBy: boolean = true;
  let trimDigestTo: number = -1;

  const trimString = (s: string, trimTo: number) =>
    trimTo === -1 ? s : s.slice(0, trimTo);
  let plotDiv: HTMLElement | undefined = undefined;
  let digest: string;

  let graph: SunburstChartInstance | undefined = undefined;

  onMount(() => {
    plotDiv = document.getElementById("plot_layers");
    if (plotDiv === undefined) {
      throw new Error("Did not get the 'plot_layers' div");
    }
  });

  $: if (data !== undefined) {
    layers = Object.keys(data);
  }

  const printLayerSize = (layerDigest: string) =>
    formatByte(data[layerDigest as keyof DataRouteReply].total_size);
  const printCreatedBy = (layerDigest: string) =>
    data[layerDigest as keyof DataRouteReply].CreatedBy;

  const drawPlot = () => {
    plotDiv.innerHTML = "";

    const root: Dir = data[digest as keyof DataRouteReply];
    graph = Sunburst();
    graph
      .excludeRoot(true)
      .minSliceAngle(0.2)
      .tooltipContent((_d, node) => `Size: <i>${formatByte(node.value)}</i>`)
      .data(dirToDataNode(root))(plotDiv);
  };
</script>

{#if data !== undefined}
  Select the layer to visualize:
  <details open>
    <summary>Configure the table display</summary>
    <label>
      <input
        type="checkbox"
        bind:checked={showCreatedBy}
        name="showCreatedBy"
      />
      Show Created By Column
    </label>
    <br />
    <label for="trimDigestTo">Trim the digest to: </label>
    <input type="number" bind:value={trimDigestTo} name="trimDigestTo" />
  </details>
  <form on:submit|preventDefault={drawPlot}>
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
              <input
                type="radio"
                bind:group={digest}
                value={layer}
                name="layer"
              />
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
    <button type="submit" disabled={digest === undefined}
      >Plot this layer</button
    >
  </form>

  <div id="plot_layers" />
{/if}
