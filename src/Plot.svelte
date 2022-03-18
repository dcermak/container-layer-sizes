<script lang="ts">
  import { onMount } from "svelte";
  import * as Plotly from "plotly.js";
  import type { DataRouteReply } from "./types";
  import { Dir, FsSize } from "./fs-tree";
  import { formatByte } from "./util";

  export let data: DataRouteReply | undefined = undefined;

  let layers: string[] = [];
  let showCreatedBy: boolean = true;
  let trimDigestTo: number = -1;

  const trimString = (s: string, trimTo: number) =>
    trimTo === -1 ? s : s.slice(0, trimTo);
  let plotDiv: HTMLElement | undefined = undefined;
  let digest: string;

  let drawPlotPromise: Promise<Plotly.PlotlyHTMLElement> | undefined =
    undefined;

  const config = { responsive: true };

  const layout = {
    margin: { l: 0, r: 0, b: 0, t: 0 }
  };

  onMount(() => {
    plotDiv = document.getElementById("plot_layers");
    if (plotDiv === undefined) {
      throw new Error("Did not get ");
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
    drawPlotPromise = (() => {
      const root: Dir = data[digest as keyof DataRouteReply];
      const fsSize = new FsSize(root, 5);

      return Plotly.newPlot(
        plotDiv,
        [
          {
            type: "sunburst",
            ...fsSize,
            outsidetextfont: { size: 20, color: "#377eb8" },
            leaf: { opacity: 0.4 },
            marker: { line: { width: 2 } },
            branchvalues: "total"
          } as Partial<Plotly.PlotData>
        ],
        layout,
        config
      );
    })();
  };
</script>

{#if data !== undefined}
  <div id="plot_config">
    Select the layer to visualize:
    <details>
      <summary>Configure the table display</summary>
      <input
        type="checkbox"
        bind:checked={showCreatedBy}
        name="showCreatedBy"
      />
      <label for="showCreatedBy">Show Created By Column</label>
      <br />
      <label for="trimDigestTo">Trim the digest to: </label>
      <input type="number" bind:value={trimDigestTo} name="trimDigestTo" />
    </details>
    <form>
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
    </form>
    <button on:click={drawPlot}>Plot this layer</button>
  </div>

  {#if drawPlotPromise !== undefined}
    {#await drawPlotPromise}
      Plotting data...
    {:catch err}
      Failed to plot the data, got {err.message}
    {/await}
  {/if}
  <div id="plot_layers" />
{/if}
