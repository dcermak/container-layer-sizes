<script lang="ts">
  import {
    BackendStorage,
    type ImageEntry,
    type ImageHistory
  } from "./backend-storage";
  import Sunburst from "sunburst-chart";
  import ContainerImageTable from "./ContainerImageTable.svelte";
  import LayerSelection from "./LayerSelection.svelte";
  import LayerSelectionConfig from "./LayerSelectionConfig.svelte";
  import type { DataRouteReply } from "./types";
  import { formatByte } from "./util";
  import {
    compareDirsToDataNodes,
    findNodeInOtherGraph,
    type Layer
  } from "./fs-tree";

  let showCreatedBy: boolean = true;
  let trimDigestTo: number = -1;

  let leftPlot: HTMLElement;
  let rightPlot: HTMLElement;

  let allImagesPromise: Promise<ImageEntry[]> | undefined = undefined;
  let imageHistoryFetchPromises: Map<
    number,
    Promise<ImageHistory> | undefined
  > = new Map();
  let selectedLayers: Record<number, Record<string, string | undefined>> = {};
  let savedHistories: Record<number, ImageHistory> = {};

  const backend = new BackendStorage();

  const fetchAllImages = (): void => {
    allImagesPromise = undefined;
    imageHistoryFetchPromises = new Map();
    allImagesPromise = backend.fetchAllImages();
  };

  const fetchHistoryOfImage = (id: number) => {
    selectedLayers[id] = {};
    imageHistoryFetchPromises.set(
      id,
      backend.fetchImageHistory(id).then((hist) => {
        savedHistories[id] = hist;
        return hist;
      })
    );
    imageHistoryFetchPromises = imageHistoryFetchPromises;
  };

  const compareLayers = () => {
    const res: string[] = [];
    const layersToCompare: Layer[] = [];

    Object.entries(selectedLayers).forEach(([id, layers]) => {
      Object.entries(layers).forEach(([digest, layer]) => {
        layersToCompare.push(
          savedHistories[id as unknown as number].History[digest].Contents[
            layer as keyof DataRouteReply
          ]
        );
        res.push(layer);
      });
    });
    if (res.length != 2) {
      alert(`You can only compare two layers, but you selected ${res.length}`);
      return;
    }

    leftPlot.innerHTML = "";
    rightPlot.innerHTML = "";

    const [dl, dr] = compareDirsToDataNodes(
      layersToCompare[0],
      layersToCompare[1]
    );

    const [gl, gr] = [
      { data: dl, div: leftPlot },
      { data: dr, div: rightPlot }
    ].map(({ data, div }) => {
      const g = Sunburst();

      g
        .excludeRoot(true)
        .minSliceAngle(0.2)
        .color((d) => d.color)
        .tooltipContent((_d, node) => `Size: <i>${formatByte(node.value)}</i>`)
        .width(window.innerWidth / 2)
        .data(data)(div);

      return g;
    });

    gl.onClick((n) => {
      gl.focusOnNode(n);
      const otherFocus = findNodeInOtherGraph(n, gr.data());
      if (otherFocus !== undefined && otherFocus !== null) {
        gr.focusOnNode(otherFocus);
      }
    });
    gr.onClick((n) => {
      gr.focusOnNode(n);
      const otherFocus = findNodeInOtherGraph(n, gl.data());
      if (otherFocus !== undefined && otherFocus !== null) {
        gl.focusOnNode(otherFocus);
      }
    });
    gr.data;
  };
</script>

<button on:click={fetchAllImages}>Fetch all images on the backend</button><br />

<div class="row">
  <div bind:this={leftPlot} class="column" />
  <div bind:this={rightPlot} class="column" />
</div>

{#if allImagesPromise !== undefined}
  {#await allImagesPromise}
    Fetching all images from the backend
  {:then allImages}
    {#if allImages.length == 0}
      No images are stored in the backend
    {:else}
      <br />
      <LayerSelectionConfig bind:showCreatedBy bind:trimDigestTo />

      <form on:submit|preventDefault={compareLayers}>
        {#each allImages as img}
          <h1>{img.Name}</h1>
          {#if imageHistoryFetchPromises.get(img.ID) === undefined}
            <button on:click={() => fetchHistoryOfImage(img.ID)}
              >Fetch the history of this image</button
            >
          {:else}
            {#await imageHistoryFetchPromises.get(img.ID)}
              Fetching the history of {img.Name}
            {:then data}
              {#each Object.entries(data.History) as entry}
                <h2>{entry[0]}</h2>
                <br />
                <ContainerImageTable
                  containerImageName={img.Name}
                  imageInfo={entry[1].InspectInfo}
                  showPullProgress={false}
                  open={false}
                />
                <br />
                <LayerSelection
                  data={entry[1].Contents}
                  {showCreatedBy}
                  {trimDigestTo}
                  bind:digest={selectedLayers[img.ID][entry[0]]}
                />
              {/each}
            {/await}
          {/if}
          <br />
        {/each}
        <button type="submit">Compare the selected layers</button>
      </form>
    {/if}
  {/await}
{/if}

<style>
  .column {
    float: left;
    width: 50%;
  }

  .row:after {
    content: "";
    display: table;
    clear: both;
  }
</style>
