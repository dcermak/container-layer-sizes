<script lang="ts">
  import Sunburst from "sunburst-chart";
  import type { SunburstChartInstance } from "sunburst-chart";
  import type { DataRouteReply } from "./types";
  import { dirToDataNode } from "./fs-tree";
  import type { Dir } from "./fs-tree";
  import { formatByte } from "./util";
  import LayerSelection from "./LayerSelection.svelte";
  import LayerSelectionConfig from "./LayerSelectionConfig.svelte";

  export let data: DataRouteReply | undefined = undefined;

  let showCreatedBy: boolean = true;
  let trimDigestTo: number = -1;

  let plotDiv: HTMLElement;
  let digest: string;

  let graph: SunburstChartInstance | undefined = undefined;

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
  <LayerSelectionConfig bind:showCreatedBy bind:trimDigestTo />
  <form on:submit|preventDefault={drawPlot}>
    <LayerSelection {data} {showCreatedBy} {trimDigestTo} bind:digest />
    <button type="submit" disabled={digest === undefined}
      >Plot this layer</button
    >
  </form>

  <div bind:this={plotDiv} />
{/if}
