import * as Plotly from "plotly.js";
import { Dir, FsSize } from "./fs-tree";
import { promiseWithTimeout, sleep } from "./util";

const enum TaskState {
  New = 0,
  Pulling,
  Extracting,
  Analyzing,
  Finished,
  Error
}

// this is the json.Marshall of github.com/containers/image/v5/types.ImageInspectInfo
interface ImageInspectInfo {
  readonly Tag: string;
  readonly Created: string;
  readonly DockerVersion: string;
  readonly Labels?: { string: string };
  readonly Architecture: string;
  readonly Variant: string;
  readonly Os: string;
  readonly Layers: string[];
  readonly Env?: string[];
}

// json.Marshall of the struct with the same name in main.go
interface LayerDownloadProgress {
  readonly total_size: number;
  readonly downloaded: number;
}

interface PullProgress {
  string: LayerDownloadProgress;
}

interface Task {
  readonly image: string;
  readonly state: TaskState;
  readonly error: string;
  readonly pull_progress?: PullProgress;
  readonly image_info?: ImageInspectInfo;
}

const layout = {
  margin: { l: 0, r: 0, b: 0, t: 0 }
};

interface DataRouteReply {
  string: Dir;
}

const enum PageState {
  New,
  Pulling,
  Plot,
  Error,
  Cancelled
}

class PageElements {
  /** default timeout to fetch an image */
  public pullTimeoutMs: number = 120 * 60 * 1000;

  public readonly plotConfig: HTMLElement;
  public readonly plot: HTMLElement;
  public readonly fetchStatus: HTMLElement;
  public readonly plotTitle: HTMLElement;
  public readonly imageInfo: HTMLElement;

  public readonly layersOption: HTMLSelectElement;

  public readonly imageNameInput: HTMLInputElement;

  public readonly layerConfirm: HTMLButtonElement;
  public readonly pullImageButton: HTMLButtonElement;
  public readonly cancelButton: HTMLButtonElement;

  public taskId?: string = undefined;
  public state: PageState = PageState.New;

  private drawImageInfoTable(
    imageInfo: ImageInspectInfo,
    imageUrl: string,
    pullProgres?: { string: LayerDownloadProgress }
  ): HTMLTableElement {
    const layerTable = document.createElement("table");
    const [layerTableHead, layerTableBody] = ["thead", "tbody"].map((tag) =>
      document.createElement(tag)
    );

    const heading1 = document.createElement("th");
    heading1.innerHTML = "property";
    const heading2 = document.createElement("th");
    heading2.innerHTML = "value";

    const row1 = document.createElement("tr");
    row1.appendChild(heading1);
    row1.appendChild(heading2);

    layerTableHead.appendChild(row1);

    const formatDownloadProgress = (prog: LayerDownloadProgress): string => {
      if (prog.total_size === -1) {
        return "starting...";
      }
      if (prog.total_size === prog.downloaded) {
        return "downloaded";
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

      return `downloading... ${(prog.downloaded / unitValue).toFixed(
        2
      )} ${unitName} of ${(prog.total_size / unitValue).toFixed(
        2
      )} ${unitName} done`;
    };

    const {
      Tag,
      DockerVersion,
      Created,
      Variant,
      Architecture,
      Os,
      Env,
      Labels,
      Layers
    } = imageInfo;

    const tableLineData = [
      ["image", imageUrl],
      ["Created", Created],
      ["Docker version", DockerVersion],
      ["Architecture", Architecture],
      ["Operating system", Os]
    ];

    [
      ["Tag", Tag],
      ["Variant", Variant]
    ].forEach(([name, value]) => {
      if (value != "") {
        tableLineData.push([name, value]);
      }
    });

    tableLineData.push([
      "Layers",
      pullProgres === undefined
        ? Layers.join("<br/>")
        : Layers.map(
            (layer) =>
              `${layer}: ${formatDownloadProgress(
                pullProgres[layer as keyof PullProgress]
              )}`
          ).join("<br/>")
    ]);

    if (Env != undefined) {
      tableLineData.push(["Environment variables", Env.join("<br/>")]);
    }
    if (Labels != undefined) {
      tableLineData.push([
        "Labels",
        Object.entries(Labels)
          .map(([lbl, val]) => `${lbl}: ${val}`)
          .join("<br/>")
      ]);
    }

    tableLineData.push();

    tableLineData.forEach(([name, value]) => {
      const line1 = document.createElement("td");
      line1.innerHTML = name;

      const line2 = document.createElement("td");
      line2.innerHTML = value;

      const row = document.createElement("tr");
      row.appendChild(line1);
      row.appendChild(line2);

      layerTableBody.appendChild(row);
    });

    layerTable.appendChild(layerTableHead);
    layerTable.appendChild(layerTableBody);

    return layerTable;
  }

  public constructor(doc: Document) {
    [
      this.plotConfig,
      this.plot,
      this.fetchStatus,
      this.plotTitle,
      this.imageInfo
    ] = ["plot_config", "plot", "fetch_status", "plot_title", "image_info"].map(
      (id) => doc.getElementById(id)!
    );

    this.imageNameInput = doc.getElementById("image_name")! as HTMLInputElement;

    [this.pullImageButton, this.layerConfirm, this.cancelButton] = [
      "pull_image_btn",
      "layer_confirm",
      "cancel_pull_analyze_btn"
    ].map((id) => doc.getElementById(id)! as HTMLButtonElement);
    this.layersOption = doc.getElementById("layers")! as HTMLSelectElement;
  }

  public setNoImage(): void {
    this.state = PageState.New;

    this.plotConfig.hidden = true;
    this.fetchStatus.hidden = true;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;
    this.imageInfo.hidden = true;
    this.imageInfo.textContent = "";

    this.cancelButton.hidden = false;
  }

  public async setPullingImage(): Promise<void> {
    this.state = PageState.Pulling;

    this.plotConfig.hidden = true;
    this.fetchStatus.hidden = false;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;
    this.cancelButton.hidden = false;

    const imageUrl = this.imageNameInput.value;
    const launchResponse = await fetch("/task", {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded"
      },
      body: `image=${imageUrl}`
    });
    this.taskId = await launchResponse.text();

    this.fetchStatus.hidden = false;
    this.fetchStatus.textContent = `Pulling image ${imageUrl}`;

    await promiseWithTimeout(async () => {
      while (true) {
        if (this.state != PageState.Pulling) {
          return;
        }
        const task: Task = await (
          await fetch(`/task?id=${this.taskId}`)
        ).json();
        switch (task.state) {
          case TaskState.New:
          case TaskState.Pulling:
            if (task.image_info != undefined) {
              this.fetchStatus.hidden = true;
              this.imageInfo.hidden = false;
              this.imageInfo.innerHTML = "";

              this.imageInfo.appendChild(
                this.drawImageInfoTable(
                  task.image_info,
                  imageUrl,
                  task.pull_progress
                )
              );
            } else {
              this.fetchStatus.hidden = false;
              this.fetchStatus.textContent = `Pulling image ${imageUrl}`;
            }
            break;
          case TaskState.Extracting:
            this.fetchStatus.hidden = false;
            this.fetchStatus.textContent = `Extracting the image`;
            break;
          case TaskState.Analyzing:
            this.fetchStatus.hidden = false;
            this.fetchStatus.textContent = `Analyzing the image ${imageUrl}`;
            break;
          case TaskState.Finished:
            return;
          case TaskState.Error:
            throw new Error(`Failed to fetch ${imageUrl}: ${task.error}`);
        }
        await sleep(1000);
      }
    }, this.pullTimeoutMs);
  }

  public setError(errMsg: string): void {
    this.state = PageState.Error;

    this.plotConfig.hidden = true;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;
    this.imageInfo.hidden = true;

    this.fetchStatus.hidden = false;
    this.fetchStatus.textContent = errMsg;
  }

  public setDisplayPlot(data: DataRouteReply): void {
    this.state = PageState.Plot;

    while (this.layersOption.options.length > 0) {
      this.layersOption.options.remove(0);
    }

    this.cancelButton.hidden = true;
    this.fetchStatus.hidden = true;

    this.plotConfig.hidden = false;

    Object.keys(data).forEach((layerDigest) => {
      const opt = document.createElement("option");
      opt.text = layerDigest;
      this.layersOption.options.add(opt);
    });

    const setPlotSize = () => {
      this.plot.style.width = `${window.innerWidth}px`;
      this.plot.style.height = `${window.innerHeight}px`;
    };
    setPlotSize();
    window.onresize = setPlotSize;

    this.layerConfirm.onclick = () => {
      const digest = this.layersOption.value;
      const root: Dir = data[digest as keyof DataRouteReply];

      this.plotTitle.textContent = `Disk usage in ${digest}`;
      this.plotTitle.hidden = false;

      const fsSize = new FsSize(root, 5);

      this.plot.hidden = false;
      Plotly.newPlot(
        this.plot,
        [
          {
            type: "sunburst",
            ...fsSize,
            outsidetextfont: { size: 20, color: "#377eb8" },
            leaf: { opacity: 0.4 },
            marker: { line: { width: 2 } },
            branchvalues: "total"
          } as Partial<Plotly.PlotData>
        ],
        layout
      );
    };
  }

  public async cancel(): Promise<void> {
    if (this.taskId === undefined) {
      throw new Error("Cancellation has been requested, but no task id exists");
    }

    this.state = PageState.Cancelled;

    this.plotConfig.hidden = true;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;
    this.imageInfo.hidden = true;

    this.fetchStatus.hidden = false;
    this.fetchStatus.textContent = "pull/analysis has been canceled";

    await fetch(`/task?id=${this.taskId}`, { method: "DELETE" });
  }
}

window.onload = (): void => {
  const pageElements = new PageElements(document);

  pageElements.setNoImage();

  pageElements.cancelButton.onclick = () => pageElements.cancel();

  pageElements.pullImageButton.onclick = async () => {
    try {
      await pageElements.setPullingImage();
    } catch (err) {
      pageElements.setError((err as Error).toString());
      return;
    }

    const data = await (await fetch(`/data?id=${pageElements.taskId}`)).json();
    pageElements.setDisplayPlot(data);
  };
};
