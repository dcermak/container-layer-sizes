<script lang="ts">
  import type { Layer } from "./fs-tree";

  import { activeTask, pageState } from "./stores";
  import { DataRouteReply, ImageInspectInfo, PageState } from "./types";

  export let data: DataRouteReply;

  let saveHistoryPromise: Promise<void> | undefined = undefined;

  const addr = "http://localhost:4040";

  interface ImageHistoryEntry {
    readonly tags: string[];
    readonly contents: { string: Layer };
    readonly inspect_info: ImageInspectInfo;
  }

  type HistoryT = { string: ImageHistoryEntry };

  interface NewImageHistory {
    readonly name: string;
    readonly History: HistoryT;
  }

  interface ImageHistory extends NewImageHistory {
    readonly ID: number;
  }

  const saveHistory = async () => {
    let existingHistory: ImageHistory | NewImageHistory | undefined = undefined;
    const imgName = $activeTask.Image.Image;
    try {
      existingHistory = await (await fetch(`${addr}?name=${imgName}`)).json();
    } catch {}

    const currentEntryKey = $activeTask.Image.ImageDigest as keyof HistoryT;
    let newHistEntry: any = {};
    newHistEntry[currentEntryKey] = {
      tags: $activeTask.Image.Tag === "" ? [] : [$activeTask.Image.Tag],
      contents: data,
      inspect_info: $activeTask.Image.ImageInfo
    };

    let entries: HistoryT = {
      ...newHistEntry,
      ...(existingHistory?.History ?? {})
    };

    let payload: NewImageHistory | ImageHistory = {
      name: existingHistory?.name ?? imgName,
      History: entries
    };
    let method = "PUT";

    if ((existingHistory as ImageHistory | undefined)?.ID !== undefined) {
      payload = { ID: (existingHistory as ImageHistory).ID, ...payload };
      method = "PUT";
    }

    await fetch(addr, {
      method,
      headers: {
        "Content-Type": "application/x-www-form-urlencoded"
      },
      body: JSON.stringify(payload)
    });
  };
</script>

<main>
  {#if $activeTask !== undefined && $activeTask.Image.ImageInfo !== undefined && $pageState === PageState.Plot}
    <button on:click={saveHistory}
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
