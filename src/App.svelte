<script lang="ts">
  import ImageInformation from "./ImageInformation.svelte";
  import Plot from "./Plot.svelte";
  import { DataRouteReply, PageState, Task, TaskState } from "./types";
  import { pageState, activeTask } from "./stores";
  import { onDestroy } from "svelte";

  let taskId: string | undefined = undefined;
  let taskStateTimer: NodeJS.Timer | undefined = undefined;
  let appStatus: PageState = PageState.New;

  //let activeTask: Task | undefined = undefined;
  let imageUrl: string | undefined = undefined;

  let dataPromise: Promise<DataRouteReply> | undefined = undefined;

  const pullImage = async () => {
    if (imageUrl === undefined || imageUrl === "") {
      alert("Please provide an image url");
      return;
    }

    taskId = await (
      await fetch("/task", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded"
        },
        body: `image=${imageUrl}`
      })
    ).text();
    pageState.set(PageState.Pulling);

    taskStateTimer = setInterval(async () => {
      let t: Task;
      try {
        t = await (await fetch(`/task?id=${taskId}`)).json();
      } catch (err) {
        activeTask.set(undefined);
        if (taskStateTimer !== undefined) {
          clearTimeout(taskStateTimer);
        }
        pageState.set(PageState.Error);
        return;
      }

      activeTask.set(t);
      if (t.state === TaskState.Finished) {
        if (taskStateTimer !== undefined) {
          clearInterval(taskStateTimer);
        }
        pageState.set(PageState.Plot);

        dataPromise = fetch(`/data?id=${taskId}`).then((r) => r.json());
      }
    }, 1000);
  };

  const unsubscribe = pageState.subscribe((state) => {
    appStatus = state;
    if (state !== PageState.Pulling) {
      if (taskStateTimer !== undefined) {
        clearTimeout(taskStateTimer);
      }
    }
  });

  const cancelPull = async () => {
    pageState.set(PageState.Cancelled);
    if (taskId === undefined)
      await fetch(`/task?id=${taskId}`, { method: "DELETE" });
  };

  onDestroy(unsubscribe);
</script>

<main>
  <div id="image_name_entry">
    Enter the image to analyze:
    <input bind:value={imageUrl} type="text" />
    <button on:click={pullImage}>Pull and analyze this image</button>
    {#if appStatus === PageState.Pulling}
      <button on:click={cancelPull}>Cancel pull/analysis</button>
    {/if}
  </div>
  {#if $pageState === PageState.Pulling}
    <p>Pulling {imageUrl}</p>
  {:else if $pageState === PageState.Error}
    <p>
      Error occurred while pulling the image{#if $activeTask !== undefined && $activeTask.error !== ""}, got
        error: {$activeTask.error}{/if}
    </p>
  {/if}
  <ImageInformation />
  {#if dataPromise !== undefined && $pageState === PageState.Plot}
    {#await dataPromise}
      <p>Fetching data...</p>
    {:then data}
      <Plot {data} />
    {:catch err}
      <p>Failed to retrieve the plot data: {err.message}</p>{/await}
  {/if}
</main>
