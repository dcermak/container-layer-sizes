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
