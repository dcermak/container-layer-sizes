import type { Layer } from "./fs-tree";

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

export interface Platform {
  readonly architecture: string;
  readonly os: string;
  readonly variant?: string;
}

export interface ExtractedDigest {
  readonly mediaType: string;
  readonly size: number;
  readonly digest: string;
  readonly platform: Platform;
}

export interface Manifest {
  readonly schemaVersion: number;
  readonly mediaType: string;
  readonly config: ExtractedDigest;
  readonly layers: readonly ExtractedDigest[];
  readonly manifests: readonly ExtractedDigest[];
}

export interface ContainerImage {
  readonly Image: string;
  readonly Tag: string;
  readonly Transport: string;
  readonly RemoteDigest: string;
  readonly Manifest: Manifest;
  readonly OciImageDigest: string;
  readonly ImageInfo: ImageInspectInfo | undefined | null;
}

export interface Task {
  readonly Image: ContainerImage;
  readonly state: TaskState;
  readonly error: string;
  readonly pull_progress: PullProgress | undefined | null;
}

export interface DataRouteReply {
  string: Layer;
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
