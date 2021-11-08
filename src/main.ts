import * as Plotly from "plotly.js";
import { Dir, FsSize } from "./fs-tree";

const layout = {
  margin: { l: 0, r: 0, b: 0, t: 0 }
};

window.onload = (): void => {
  const [
    plotConfigElement,
    plotElement,
    layerConfirmElement,
    layersOption,
    imageNameInputElement,
    pullImageButton,
    plotTitleElement
  ] = [
    "plot_config",
    "plot",
    "layer_confirm",
    "layers",
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
