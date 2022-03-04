import type { Dir } from "./fs-tree";

// this is the json.Marshall of github.com/containers/image/v5/types.ImageInspectInfo
export interface ImageInspectInfo {
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
export interface LayerDownloadProgress {
  readonly total_size: number;
  readonly downloaded: number;
}

export interface PullProgress {
  string: LayerDownloadProgress;
}

export interface Task {
  readonly image: string;
  readonly state: TaskState;
  readonly error: string;
  readonly pull_progress: PullProgress | undefined | null;
  readonly image_info: ImageInspectInfo | undefined | null;
}

export interface DataRouteReply {
  string: Dir;
}

export const enum PageState {
  New,
  Pulling,
  Plot,
  Error,
  Cancelled
}

export const enum TaskState {
  New = 0,
  Pulling,
  Extracting,
  Analyzing,
  Finished,
  Error
}
