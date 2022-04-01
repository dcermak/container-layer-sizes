<script lang="ts">
  import {
    BackendStorage,
    type ImageEntry,
    type ImageHistory
  } from "./backend-storage";
  import ContainerImageTable from "./ContainerImageTable.svelte";

  let allImagesPromise: Promise<ImageEntry[]> | undefined = undefined;
  let imageHistoryFetchPromises: Map<
    number,
    Promise<ImageHistory> | undefined
  > = new Map();

  const backend = new BackendStorage();

  const fetchAllImages = (): void => {
    allImagesPromise = undefined;
    imageHistoryFetchPromises = new Map();
    allImagesPromise = backend.fetchAllImages();
  };

  const fetchHistoryOfImage = (id: number) => {
    imageHistoryFetchPromises.set(id, backend.fetchImageHistory(id));
    imageHistoryFetchPromises = imageHistoryFetchPromises;
  };
</script>

<button on:click={fetchAllImages}>Fetch all images on the backend</button><br />

{#if allImagesPromise !== undefined}
  {#await allImagesPromise}
    Fetching all images from the backend
  {:then allImages}
    {#if allImages.length == 0}
      No images are stored in the backend
    {:else}
      {#each allImages as img}
        {img.Name}
        {#if imageHistoryFetchPromises.get(img.ID) === undefined}
          <button on:click={() => fetchHistoryOfImage(img.ID)}
            >Fetch the history of this image</button
          >
        {:else}
          {#await imageHistoryFetchPromises.get(img.ID)}
            Fetching the history of {img.Name}
          {:then data}
            {#each Object.entries(data.History) as entry}
              {entry[0]}<br />
              <ContainerImageTable
                containerImageName={img.Name}
                imageInfo={entry[1].InspectInfo}
                showPullProgress={false}
                open={false}
              />
            {/each}
          {/await}
        {/if}
        <br />
      {/each}
    {/if}
  {/await}
{/if}
