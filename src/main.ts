import * as Plotly from "plotly.js";

var layout = {
  margin: { l: 0, r: 0, b: 0, t: 0 }
};

interface Dir {
  path: string;
  total_size: number;
  files: { string: number };
  directories: { string: Dir };
}

interface IFsSize {
  readonly labels: string[];
  readonly values: number[];
  readonly ids: string[];
  readonly parents: string[];
}

class FsSize implements IFsSize {
  readonly labels: string[] = [];
  readonly values: number[] = [];
  readonly ids: string[] = [];
  readonly parents: string[] = [];

  constructor(rootDir: Dir, maxDepth: number = -1) {
    insertDirRec(rootDir, "", this, 0, maxDepth);
  }
}

function insertDirRec(
  dir: Dir,
  prefix: string,
  fsSize: IFsSize,
  depth: number,
  maxDepth: number = -1
): void {
  if (maxDepth > 0 && depth > maxDepth) {
    return;
  }
  Object.entries(dir.files).forEach(([fname, size]) => {
    fsSize.labels.push(fname);
    fsSize.values.push(size);
    fsSize.parents.push(prefix);
    fsSize.ids.push(`${prefix}/${fname}`);
  });

  Object.entries(dir.directories).forEach(([dirName, subdir]) => {
    fsSize.labels.push(dirName);
    fsSize.values.push(subdir.total_size);
    fsSize.parents.push(prefix);
    fsSize.ids.push(`${prefix}/${dirName}`);

    insertDirRec(
      subdir,
      `${prefix}/${subdir.path}`,
      fsSize,
      depth + 1,
      maxDepth
    );
  });
}

window.onload = (): void => {
  const [
    plotConfigElement,
    plotElement,
    layerConfirmElement,
    layersOption,
    _imageNameEntryElement,
    imageNameInputElement,
    pullImageButton,
    plotTitleElement
  ] = [
    "plot_config",
    "plot",
    "layer_confirm",
    "layers",
    "image_name_entry",
    "image_name",
    "pull_image_btn",
    "plot_title"
  ].map((id) => document.getElementById(id)!);

  const layersOptionElement = layersOption as HTMLSelectElement;
  layersOptionElement.multiple = false;

  pullImageButton.onclick = async () => {
    plotElement.hidden = true;
    plotConfigElement.hidden = true;
    for (let i = 0; i < layersOptionElement.options.length; i++) {
      layersOptionElement.options.remove(0);
    }
    plotTitleElement.hidden = true;

    const response = await fetch(
      `/data/${(imageNameInputElement as HTMLInputElement).value}`
    );
    const data = await response.json();

    plotConfigElement.hidden = false;

    Object.keys(data).forEach((layerDigest) => {
      const opt = document.createElement("option");
      opt.text = layerDigest;
      layersOptionElement.options.add(opt);
    });

    const setPlotSize = () => {
      plotElement.style.width = `${window.innerWidth}px`;
      plotElement.style.height = `${window.innerHeight}px`;
    };
    setPlotSize();
    window.onresize = setPlotSize;

    (layerConfirmElement as HTMLButtonElement).onclick = () => {
      const digest = layersOptionElement.value;
      const root: Dir = data[digest];

      plotTitleElement.textContent = `Disk usage in ${digest}`;
      plotTitleElement.hidden = false;

      const fsSize = new FsSize(root, 5);

      plotElement.hidden = false;
      Plotly.newPlot(
        plotElement,
        [
          {
            type: "sunburst",
            ...fsSize,
            outsidetextfont: { size: 20, color: "#377eb8" },
            leaf: { opacity: 0.4 },
            marker: { line: { width: 2 } }
          } as Partial<Plotly.PlotData>
        ],
        layout
      );
    };
  };
};
