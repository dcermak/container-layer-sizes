<script lang="ts">
  import ImageInformation from "./ImageInformation.svelte";
  import Plot from "./Plot.svelte";
  import { type ExtractedDigest, PageState, TaskState } from "./types";
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
    { value: Transport.DOCKER, description: "Container registry" },
    { value: Transport.DOCKER_ARCHIVE, description: "Local docker archive" },
    {
      value: Transport.CONTAINERS_STORAGE,
      description: "Local podman container image"
    },
    { value: Transport.BACKEND, description: "Previously analyzed image" }
  ];

  let dataPromise: Promise<DataRouteReply> | undefined = undefined;

  let imageReadyToAnalyze: boolean = false;
  $: {
    if (transport === Transport.DOCKER) {
      imageReadyToAnalyze = chosenPlatform !== undefined;
    } else {
      imageReadyToAnalyze = true;
    }
  }

  let platformDigestsPromise: Promise<ExtractedDigest[]> | undefined =
    undefined;
  let chosenPlatform: ExtractedDigest | undefined = undefined;

  const fetchPlatformDigests = () => {
    chosenPlatform = undefined;
    platformDigestsPromise = undefined;

    if (imageUrl === undefined || imageUrl === "") {
      alert("Please provide an image url");
      return;
    }

    platformDigestsPromise = fetch(`/image?url=${transport}${imageUrl}`).then(
      (resp) => {
        if (!resp.ok) {
          throw new Error(resp.statusText);
        }
        return resp.json();
      }
    );
  };

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

      let fullUrl = transport + imageUrl;

      // for docker transports, the user might select an architecture, append
      // its digest to the url with a "@", but only if the user did not specify
      // a specific digest themselves
      if (
        chosenPlatform !== undefined &&
        transport === Transport.DOCKER &&
        imageUrl.indexOf("@") === -1
      ) {
        // if the url includes a tag, we need to drop it, as we can only set a
        // tag or a digest, not both
        if (imageUrl.indexOf(":") !== -1) {
          fullUrl =
            transport + imageUrl.split(":")[0] + "@" + chosenPlatform.digest;
        } else {
          fullUrl = transport + imageUrl + "@" + chosenPlatform.digest;
        }
      }

      const resp = await fetch("/task", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded"
        },
        body: `image=${fullUrl}`
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
  {#if transport === Transport.DOCKER}
    <button on:click={fetchPlatformDigests} type="button"
      >Retrieve available platforms</button
    >
    {#if platformDigestsPromise !== undefined}
      {#await platformDigestsPromise then platformDigest}
        <select bind:value={chosenPlatform}>
          {#each platformDigest as digest}
            <option value={digest}>
              {digest.platform
                .architecture}{#if digest.platform.variant !== undefined && digest.platform.variant !== ""}/{digest
                  .platform.variant}{/if}
              on {digest.platform.os}
            </option>
          {/each}
        </select>
      {:catch err}
        Failed to fetch the list of platforms, got {err}
      {/await}
    {/if}
  {/if}
  {#if imageReadyToAnalyze}
    <button type="submit">Pull and analyze this image</button>
    {#if appStatus === PageState.Pulling}
      <button on:click={cancelPull}>Cancel pull/analysis</button>
    {/if}
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
