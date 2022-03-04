<script lang="ts">
  import { onMount } from "svelte";
  import * as Plotly from "plotly.js";
  import type { DataRouteReply } from "./types";
  import { Dir, FsSize } from "./fs-tree";

  export let data: DataRouteReply | undefined = undefined;

  let layers: string[] = [];
  let plotDiv: HTMLElement | undefined = undefined;
  let digest: string;

  let drawPlotPromise: Promise<Plotly.PlotlyHTMLElement> | undefined =
    undefined;

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
        layout
      );
    })();
  };
</script>

{#if data !== undefined}
  <div id="plot_config">
    Select the layer to visualize:
    <select bind:value={digest}>
      {#each layers as layer}
        <option value={layer}>{layer}</option>
      {/each}
    </select>
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
