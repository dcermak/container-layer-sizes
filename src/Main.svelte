<script lang="ts">
  import ImageInformation from "./ImageInformation.svelte";
  import Plot from "./Plot.svelte";
  import { PageState, TaskState } from "./types";
  import type { DataRouteReply, Task } from "./types";
  import Storage from "./Storage.svelte";
  import { pageState, activeTask } from "./stores";
  import { onDestroy } from "svelte";

  let taskId: string | undefined = undefined;
  let taskStateTimer: NodeJS.Timer | undefined = undefined;
  let appStatus: PageState = PageState.New;

  let imageUrl: string | undefined = undefined;
  let transport: Transport | undefined = undefined;
  let errorMsg: string | undefined = undefined;

  const enum Transport {
    DOCKER = "docker://",
    DOCKER_ARCHIVE = "docker-archive:",
    CONTAINERS_STORAGE = "containers-storage:",
    BACKEND = "backend",
    UPLOAD_ARCHIVE = "upload-docker-archive"
  }

  const TRANSPORTS = [
    { value: Transport.DOCKER, description: "Container Registry" },
    { value: Transport.DOCKER_ARCHIVE, description: "local docker archive" },
    {
      value: Transport.CONTAINERS_STORAGE,
      description: "local podman container image"
    },
    { value: Transport.BACKEND, description: "previously analyzed image" }
  ];

  let dataPromise: Promise<DataRouteReply> | undefined = undefined;

  const pullImage = async () => {
    if (imageUrl === undefined || imageUrl === "") {
      alert("Please provide an image url");
      return;
    }
    errorMsg = undefined;
    taskId = undefined;
    activeTask.set(undefined);

    try {
      if (
        transport === Transport.BACKEND ||
        transport == Transport.UPLOAD_ARCHIVE
      ) {
        alert("The backend and upload sources are not yet supported");
      }
      const resp = await fetch("/task", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded"
        },
        body: `image=${transport}${imageUrl}`
      });
      if (resp.status !== 200) {
        throw new Error(await resp.text());
      }
      taskId = await resp.text();
    } catch (err) {
      errorMsg = err.toString();
      pageState.set(PageState.Error);
    }
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
      if (t.state === TaskState.Finished || t.state === TaskState.Error) {
        if (taskStateTimer !== undefined) {
          clearInterval(taskStateTimer);
        }
        if (t.state === TaskState.Finished) {
          pageState.set(PageState.Plot);

          dataPromise = fetch(`/data?id=${taskId}`).then((r) => r.json());
        } else if (t.state === TaskState.Error) {
          pageState.set(PageState.Error);
        }
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

<form on:submit|preventDefault={pullImage}>
  Image Source:
  <select bind:value={transport}>
    {#each TRANSPORTS as transport}
      <option value={transport.value}>
        {transport.description}
      </option>
    {/each}
  </select>
  Image to analyze:
  {#if transport == Transport.UPLOAD_ARCHIVE}
    <input bind:value={imageUrl} type="file" />
  {:else}
    <input bind:value={imageUrl} type="text" />
  {/if}
  <button type="submit">Pull and analyze this image</button>
  {#if appStatus === PageState.Pulling}
    <button on:click={cancelPull}>Cancel pull/analysis</button>
  {/if}
</form>

<br />
{#if $pageState === PageState.Pulling}
  <p>Pulling and analyzing {imageUrl}</p>
{:else if $pageState === PageState.Error}
  <p>
    Error occurred while pulling the image{#if $activeTask !== undefined && $activeTask.error !== ""},
      got error: {$activeTask.error}
    {:else if errorMsg !== undefined}, got error: {errorMsg}
    {/if}
  </p>
{/if}
<ImageInformation /><br />
{#if dataPromise !== undefined && $pageState === PageState.Plot}
  {#await dataPromise}
    <p>Fetching data...</p>
  {:then data}
    <Plot {data} /><br />
    <Storage {data} />
  {:catch err}
    <p>Failed to retrieve the plot data: {err.message}</p>{/await}
{/if}
