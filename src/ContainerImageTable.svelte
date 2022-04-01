<script lang="ts">
  import type { ImageInspectInfo, PullProgress } from "./types";
  import { formatByte } from "./util";

  export let containerImageName: string;
  export let imageInfo: ImageInspectInfo;
  export let pullProgress: PullProgress | undefined | null = undefined;
  export let showPullProgress: boolean = false;
  export let open: boolean = true;

  const formatDownloadProgress = (
    pullProgress: PullProgress,
    layer: string
  ): string => {
    const prog = pullProgress[layer as keyof PullProgress];

    if (prog.total_size === -1) {
      return "starting...";
    }

    if (prog.total_size === prog.downloaded) {
      return `downloaded ${formatByte(prog.total_size)}`;
    }

    return `downloading... ${formatByte(prog.downloaded)} of ${formatByte(
      prog.total_size
    )} done`;
  };
</script>

<details {open}>
  <summary>Image Properties:</summary>
  <table>
    <thead>
      <td>property</td>
      <td>value</td>
    </thead>
    <tbody>
      <tr>
        <td>Image</td>
        <td>{containerImageName}</td>
      </tr>
      <tr>
        <td>Created</td>
        <td>{imageInfo.Created}</td>
      </tr>
      <tr>
        <td>Docker Version</td>
        <td>{imageInfo.DockerVersion}</td>
      </tr>
      <tr>
        <td>Architecture</td>
        <td>{imageInfo.Architecture}</td>
      </tr>
      <tr>
        <td>Operating System</td>
        <td>{imageInfo.Os}</td>
      </tr>
      {#if imageInfo.Tag !== ""}
        <tr>
          <td>Tag</td>
          <td>{imageInfo.Tag}</td>
        </tr>
      {/if}
      {#if imageInfo.Variant !== ""}
        <tr>
          <td>Variant</td>
          <td>{imageInfo.Variant}</td>
        </tr>
      {/if}
      {#if showPullProgress && pullProgress !== undefined && pullProgress !== null}
        <tr>
          <td>Layers</td>
          <td>
            {#each imageInfo.Layers as layer}
              {layer}: {formatDownloadProgress(pullProgress, layer)}<br />
            {/each}
          </td>
        </tr>
      {/if}
      {#if imageInfo.Env !== undefined && imageInfo.Env !== null && imageInfo.Env.length > 0}
        <tr>
          <td>Environment Variables</td>
          <td>
            {#each imageInfo.Env as env}
              {env}<br />
            {/each}
          </td>
        </tr>
      {/if}
      {#if imageInfo.Labels !== undefined && imageInfo.Labels !== null && Object.entries(imageInfo.Labels).length > 0}
        <tr>
          <td>Labels</td>
          <td>
            {#each Object.entries(imageInfo.Labels) as [lbl, val]}
              {lbl}: {val}<br />
            {/each}
          </td>
        </tr>
      {/if}
    </tbody>
  </table>
</details>
