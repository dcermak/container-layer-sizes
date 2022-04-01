import type { Layer } from "./fs-tree";

import type { ContainerImage, DataRouteReply, ImageInspectInfo } from "./types";

export interface ImageEntry {
  readonly ID: number;
  readonly Name: string;
}

interface ImageHistoryEntry {
  readonly Tags: string[];
  readonly Contents: { string: Layer };
  readonly InspectInfo: ImageInspectInfo;
}

type HistoryT = Record<string, ImageHistoryEntry>; //{ string: ImageHistoryEntry };

interface NewImageHistory {
  readonly Name: string;
  readonly History: HistoryT;
}

export interface ImageHistory extends NewImageHistory {
  readonly ID: number;
}

export class BackendStorage {
  public constructor(readonly addr: string = "http://localhost:4040") {}

  public async fetchAllImages(): Promise<ImageEntry[]> {
    const resp = await fetch(this.addr);
    if (resp.status !== 200) {
      const msg = await resp.text();
      throw new Error(
        `Failed to fetch all images, got status ${resp.status} and body: ${msg}`
      );
    }

    return await resp.json();
  }

  public async fetchImageHistory(name: string): Promise<ImageHistory>;
  public async fetchImageHistory(id: number): Promise<ImageHistory>;
  public async fetchImageHistory(
    nameOrId: string | number
  ): Promise<ImageHistory> {
    const param = `${typeof nameOrId === "string" ? "name" : "id"}=${nameOrId}`;
    const route = `${this.addr}?${param}`;

    const resp = await fetch(route);
    if (resp.status !== 200) {
      const msg = await resp.text();
      throw new Error(
        `Failed to fetch the image with ${param}, got status ${resp.status} and body: ${msg}`
      );
    }
    return await resp.json();
  }

  public async saveHistory(
    image: ContainerImage,
    data: DataRouteReply
  ): Promise<void> {
    let historyWithMatchingName: ImageHistory[] | undefined = undefined;

    try {
      historyWithMatchingName = await (
        await fetch(`${this.addr}?name=${image.Image}`)
      ).json();
    } catch {}

    const currentEntryKey = image.ImageDigest;

    const existingHistory =
      historyWithMatchingName !== undefined &&
      historyWithMatchingName.length > 0
        ? historyWithMatchingName[0]
        : undefined;

    let newHistEntry: ImageHistoryEntry = {
      Tags: image.Tag === "" ? [] : [image.Tag],
      Contents: data,
      InspectInfo: image.ImageInfo
    };

    let entries: HistoryT; // = existingHistory?.History ?? {};

    if (
      existingHistory !== undefined &&
      existingHistory.History[currentEntryKey] !== undefined
    ) {
      const oldEntry = existingHistory.History[currentEntryKey];
      const { Contents, InspectInfo } = newHistEntry;
      existingHistory.History[currentEntryKey] = {
        Tags: [...new Set([...oldEntry.Tags, ...newHistEntry.Tags])],
        Contents,
        InspectInfo
      };
      entries = existingHistory.History;
    } else {
      entries = existingHistory?.History ?? {};
      entries[currentEntryKey] = newHistEntry;
    }

    let payload: NewImageHistory | ImageHistory = {
      Name: existingHistory?.Name ?? image.Image,
      History: entries
    };
    let method = "PUT";

    if (existingHistory?.ID !== undefined) {
      payload = { ID: existingHistory.ID, ...payload };
      method = "POST";
    }

    await fetch(this.addr, {
      method,
      headers: {
        "Content-Type": "application/x-www-form-urlencoded"
      },
      body: JSON.stringify(payload)
    });
  }
}
