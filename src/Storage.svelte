<script lang="ts">
  import type { DataRouteReply } from "./types";
  import { BackendStorage } from "./backend-storage";

  import { activeTask, pageState } from "./stores";
  import { PageState } from "./types";

  export let data: DataRouteReply;

  let saveHistoryPromise: Promise<void> | undefined = undefined;

  const backend = new BackendStorage();

  const saveCurrentHistory = async () => {
    await backend.saveHistory($activeTask.Image, data);
  };
</script>

<main>
  {#if $activeTask !== undefined && $activeTask.Image.ImageInfo !== undefined && $pageState === PageState.Plot}
    <button on:click={saveCurrentHistory}
      >Save the history of this image in the backend</button
    >
    {#if saveHistoryPromise !== undefined}
      {#await saveHistoryPromise}
        Saving the image in the backend
      {:then}
        Image was saved successfully
      {:catch err}
        Failed to save the image, got {err.message}
      {/await}
    {/if}
  {/if}
</main>
