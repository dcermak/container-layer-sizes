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
    {#if currentTask !== undefined && currentTask.image_info !== undefined && currentTask.image_info !== null}
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
            <td>{currentTask.image_info.Created}</td>
          </tr>
          <tr>
            <td>Docker Version</td>
            <td>{currentTask.image_info.DockerVersion}</td>
          </tr>
          <tr>
            <td>Architecture</td>
            <td>{currentTask.image_info.Architecture}</td>
          </tr>
          <tr>
            <td>Operating System</td>
            <td>{currentTask.image_info.Os}</td>
          </tr>
          {#if currentTask.image_info.Tag !== ""}
            <tr>
              <td>Tag</td>
              <td>{currentTask.image_info.Tag}</td>
            </tr>
          {/if}
          {#if currentTask.image_info.Variant !== ""}
            <tr>
              <td>Variant</td>
              <td>{currentTask.image_info.Variant}</td>
            </tr>
          {/if}
          {#if currentTask.pull_progress !== undefined && currentTask.pull_progress !== null}
            <tr>
              <td>Layers</td>
              <td>
                {#each currentTask.image_info.Layers as layer}
                  {layer}: {formatDownloadProgress(
                    currentTask.pull_progress,
                    layer
                  )}<br />
                {/each}
              </td>
            </tr>
          {/if}
          {#if currentTask.image_info.Env !== undefined && currentTask.image_info.Env !== null && currentTask.image_info.Env.length > 0}
            <tr>
              <td>Environment Variables</td>
              <td>
                {#each currentTask.image_info.Env as env}
                  {env}<br />
                {/each}
              </td>
            </tr>
          {/if}
          {#if currentTask.image_info.Labels !== undefined && currentTask.image_info.Labels !== null && Object.entries(currentTask.image_info.Labels).length > 0}
            <tr>
              <td>Labels</td>
              <td>
                {#each Object.entries(currentTask.image_info.Labels) as [lbl, val]}
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
