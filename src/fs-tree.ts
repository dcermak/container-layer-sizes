import type { Node } from "sunburst-chart";

/**
 * Directory size as produced by the backend.
 *
 * This interface is the equivalent of the Dir struct from `dir.go`.
 */
export interface Dir {
  readonly dirname: string;
  readonly total_size: number;
  readonly files: { string: number } | {};
  readonly directories: { string: Dir } | {};
}

export interface Layer extends Dir {
  readonly CreatedBy: string;
}

export function dirToDataNode(dir: Dir): Node {
  let d = {
    name: dir.dirname,
    value: 0,
    children: [] as Node[]
  };

  Object.entries(dir.files).forEach(([fname, size]) => {
    d.children.push({ name: fname, value: size });
  });

  Object.entries(dir.directories).forEach(([_dirName, subdir]) => {
    d.children.push(dirToDataNode(subdir));
  });

  return d;
}
