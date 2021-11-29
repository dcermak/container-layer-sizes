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

interface Task {
  readonly image: string;
  readonly state: TaskState;
  readonly error: string;
}

const layout = {
  margin: { l: 0, r: 0, b: 0, t: 0 }
};

interface DataRouteReply {
  string: Dir;
}

class PageElements {
  /** default timeout to fetch an image */
  public pullTimeoutMs: number = 120 * 60 * 1000;

  public readonly plotConfig: HTMLElement;
  public readonly plot: HTMLElement;
  public readonly layerConfirm: HTMLButtonElement;
  public readonly layersOption: HTMLSelectElement;
  public readonly imageNameInput: HTMLInputElement;
  public readonly pullImageButton: HTMLElement;
  public readonly fetchStatus: HTMLElement;
  public readonly plotTitle: HTMLElement;

  public constructor(doc: Document) {
    [
      this.plotConfig,
      this.plot,
      this.pullImageButton,
      this.fetchStatus,
      this.plotTitle
    ] = [
      "plot_config",
      "plot",
      "pull_image_btn",
      "fetch_status",
      "plot_title"
    ].map((id) => doc.getElementById(id)!);

    this.imageNameInput = doc.getElementById("image_name")! as HTMLInputElement;
    this.layerConfirm = doc.getElementById(
      "layer_confirm"
    )! as HTMLButtonElement;
    this.layersOption = doc.getElementById("layers")! as HTMLSelectElement;
  }

  public setNoImage(): void {
    this.plotConfig.hidden = true;
    this.fetchStatus.hidden = true;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;
  }

  public async setPullingImage(/*imageUrl: string*/): Promise<string> {
    this.plotConfig.hidden = true;
    this.fetchStatus.hidden = false;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;

    const imageUrl = this.imageNameInput.value;
    const launchResponse = await fetch("/task", {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded"
      },
      body: `image=${imageUrl}`
    });
    const taskId = await launchResponse.text();

    this.fetchStatus.hidden = false;
    this.fetchStatus.textContent = `Pulling image ${imageUrl}`;

    await promiseWithTimeout(async () => {
      while (true) {
        const task: Task = await (await fetch(`/task?id=${taskId}`)).json();
        switch (task.state) {
          case TaskState.New:
          case TaskState.Pulling:
            this.fetchStatus.textContent = `Pulling image ${imageUrl}`;
            break;
          case TaskState.Extracting:
            this.fetchStatus.textContent = `Extracting the image`;
            break;
          case TaskState.Analyzing:
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

    return taskId;
  }

  public setError(errMsg: string): void {
    this.plotConfig.hidden = true;
    this.plotTitle.hidden = true;
    this.plot.hidden = true;

    this.fetchStatus.hidden = false;
    this.fetchStatus.textContent = errMsg;
  }

  public setDisplayPlot(data: DataRouteReply): void {
    while (this.layersOption.options.length > 0) {
      this.layersOption.options.remove(0);
    }

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
}

window.onload = (): void => {
  const pageElements = new PageElements(document);

  pageElements.setNoImage();

  pageElements.pullImageButton.onclick = async () => {
    let taskId: string;

    try {
      taskId = await pageElements.setPullingImage();
    } catch (err) {
      pageElements.setError((err as Error).toString());
      return;
    }

    const data = await (await fetch(`/data?id=${taskId}`)).json();
    pageElements.setDisplayPlot(data);
  };
};
