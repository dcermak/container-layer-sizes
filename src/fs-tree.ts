import type { DataNode, Node } from "sunburst-chart";

/**
 * Directory size as produced by the backend.
 *
 * This interface is the equivalent of the Dir struct from `dir.go`.
 */
export interface Dir {
  readonly dirname: string;
  readonly total_size: number;
  readonly files: Record<string, number>;
  readonly directories: Record<string, Dir>;
}

export interface Layer extends Dir {
  readonly CreatedBy: string;
}

const colorOfSizeDif = (
  sizeL: number,
  sizeR?: number
): "red" | "green" | "blue" | "yellow" => {
  if (sizeR === undefined) {
    return "blue";
  } else if (sizeL === sizeR) {
    return "yellow";
  } else if (sizeL > sizeR) {
    return "red";
  } else {
    return "green";
  }
};

const getColorOfDentry = (
  dirL: Dir,
  dirR?: Dir
): "red" | "green" | "blue" | "yellow" => {
  return colorOfSizeDif(dirL.total_size, dirR?.total_size);
};

const getColorOfFileEntry = (
  fname: string,
  dirL: Dir,
  dirR?: Dir
): "red" | "green" | "blue" | "yellow" => {
  return colorOfSizeDif(dirL.files[fname], dirR?.files[fname]);
};

export function compareDirsToDataNodes(dirL?: Dir, dirR?: Dir): [Node?, Node?] {
  let dL =
    dirL === undefined
      ? undefined
      : {
          name: dirL.dirname,
          value: 0,
          children: [] as Node[],
          color: getColorOfDentry(dirL, dirR)
        };
  let dR =
    dirR === undefined
      ? undefined
      : {
          name: dirR.dirname,
          value: 0,
          children: [] as Node[],
          color: getColorOfDentry(dirR, dirL)
        };

  Object.entries(dirL?.files ?? []).forEach(([fname, size]) => {
    dL.children.push({
      name: fname,
      value: size,
      color: getColorOfFileEntry(fname, dirL, dirR)
    });
  });

  Object.entries(dirR?.files ?? []).forEach(([fname, size]) => {
    dR.children.push({
      name: fname,
      value: size,
      color: getColorOfFileEntry(fname, dirR, dirL)
    });
  });

  Object.entries(dirL?.directories ?? []).forEach(([dirName, subdir]) => {
    const [left, _right] = compareDirsToDataNodes(
      subdir,
      dirR?.directories[dirName]
    );
    if (left !== undefined) {
      dL.children.push(left);
    }
  });

  Object.entries(dirR?.directories ?? []).forEach(([dirName, subdir]) => {
    const [right, _left] = compareDirsToDataNodes(
      subdir,
      dirL?.directories[dirName]
    );
    if (right !== undefined) {
      dR.children.push(right);
    }
  });

  return [dL, dR];
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

export function findNodeInOtherGraph(
  nodeLeft: Node | null | undefined,
  rootRight: Node | null | undefined
): Node | undefined {
  if (nodeLeft === null || nodeLeft === undefined) {
    return undefined;
  }

  const path: string[] = [];
  let n: DataNode | undefined | null = nodeLeft?.__dataNode;

  while (n !== null && n !== undefined) {
    path.push(n.data.name);
    n = n.parent;
  }

  let res: Node | null = rootRight.__dataNode;
  for (let i = path.length - 1; i > 0; i--) {
    if (res === null) {
      return undefined;
    }
    res = res.children.find((c) => c.data.name === path[i - 1]);
  }

  return res?.data;
}
