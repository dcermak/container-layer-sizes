<script lang="ts">
  import { pageState, activeTask } from "./stores";
  import type { PullProgress, Task } from "./types";
  import { PageState } from "./types";
  import { onDestroy } from "svelte";

  let currentTask: Task | undefined = undefined;

  const unsubActiveTask = activeTask.subscribe((t) => {
    currentTask = t;
  });
  onDestroy(unsubActiveTask);

  const formatDownloadProgress = (
    pullProgress: PullProgress,
    layer: string
  ): string => {
    const prog = pullProgress[layer as keyof PullProgress];

    if (prog.total_size === -1) {
      return "starting...";
    }

    let prefix = Math.floor(Math.log2(prog.total_size) / 10);
    let unitName = new Map([
      [0, "Byte"],
      [1, "KiB"],
      [2, "MiB"],
      [3, "GiB"],
      [4, "TiB"]
    ]).get(prefix);

    if (unitName === undefined) {
      // this should never happen…
      prefix = 4;
      unitName = "TiB";
    }
    const unitValue = 2 ** (10 * prefix);

    if (prog.total_size === prog.downloaded) {
      return `downloaded ${(prog.total_size / unitValue).toFixed(
        2
      )} ${unitName}`;
    }

    return `downloading... ${(prog.downloaded / unitValue).toFixed(
      2
    )} ${unitName} of ${(prog.total_size / unitValue).toFixed(
      2
    )} ${unitName} done`;
  };
</script>

<main>
  {#if $pageState === PageState.Plot || $pageState === PageState.Pulling}
    {#if currentTask !== undefined && currentTask.ImageInfo !== undefined && currentTask.ImageInfo !== null}
      <table>
        <thead>
          <td>property</td>
          <td>value</td>
        </thead>
        <tbody>
          <tr>
            <td>Image</td>
            <td>{currentTask.image}</td>
          </tr>
          <tr>
            <td>Created</td>
            <td>{currentTask.ImageInfo.Created}</td>
          </tr>
          <tr>
            <td>Docker Version</td>
            <td>{currentTask.ImageInfo.DockerVersion}</td>
          </tr>
          <tr>
            <td>Architecture</td>
            <td>{currentTask.ImageInfo.Architecture}</td>
          </tr>
          <tr>
            <td>Operating System</td>
            <td>{currentTask.ImageInfo.Os}</td>
          </tr>
          {#if currentTask.ImageInfo.Tag !== ""}
            <tr>
              <td>Tag</td>
              <td>{currentTask.ImageInfo.Tag}</td>
            </tr>
          {/if}
          {#if currentTask.ImageInfo.Variant !== ""}
            <tr>
              <td>Variant</td>
              <td>{currentTask.ImageInfo.Variant}</td>
            </tr>
          {/if}
          {#if $pageState === PageState.Pulling && currentTask.pull_progress !== undefined && currentTask.pull_progress !== null}
            <tr>
              <td>Layers</td>
              <td>
                {#each currentTask.ImageInfo.Layers as layer}
                  {layer}: {formatDownloadProgress(
                    currentTask.pull_progress,
                    layer
                  )}<br />
                {/each}
              </td>
            </tr>
          {/if}
          {#if currentTask.ImageInfo.Env !== undefined && currentTask.ImageInfo.Env !== null && currentTask.ImageInfo.Env.length > 0}
            <tr>
              <td>Environment Variables</td>
              <td>
                {#each currentTask.ImageInfo.Env as env}
                  {env}<br />
                {/each}
              </td>
            </tr>
          {/if}
          {#if currentTask.ImageInfo.Labels !== undefined && currentTask.ImageInfo.Labels !== null && Object.entries(currentTask.ImageInfo.Labels).length > 0}
            <tr>
              <td>Labels</td>
              <td>
                {#each Object.entries(currentTask.ImageInfo.Labels) as [lbl, val]}
                  {lbl}: {val}<br />
                {/each}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    {/if}
  {/if}
</main>
