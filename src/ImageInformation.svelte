<script lang="ts">
  import { pageState, activeTask } from "./stores";
  import type { Task } from "./types";
  import { PageState } from "./types";
  import { onDestroy } from "svelte";
  import ContainerImageTable from "./ContainerImageTable.svelte";

  let currentTask: Task | undefined = undefined;

  const unsubActiveTask = activeTask.subscribe((t) => {
    currentTask = t;
  });
  onDestroy(unsubActiveTask);
</script>

<main>
  {#if $pageState === PageState.Plot || $pageState === PageState.Pulling}
    {#if currentTask !== undefined && currentTask.Image.ImageInfo !== undefined && currentTask.Image.ImageInfo !== null}
      <ContainerImageTable
        containerImageName={currentTask.Image.Image}
        imageInfo={currentTask.Image.ImageInfo}
        pullProgress={currentTask.pull_progress}
        showPullProgress={$pageState === PageState.Pulling}
      />
    {/if}
  {/if}
</main>
