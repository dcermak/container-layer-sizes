/**
 * Directory size as produced by the backend.
 *
 * This interface is the equivalent of the Dir struct from `dir.go`.
 */
export interface Dir {
  dirname: string;
  total_size: number;
  files: { string: number } | {};
  directories: { string: Dir } | {};
}

interface IFsSize {
  readonly labels: string[];
  readonly values: number[];
  readonly ids: string[];
  readonly parents: string[];
}

/**
 * Class containing the data for visualizing the file system usage in a sunburst plot.
 */
export class FsSize implements IFsSize {
  /**
   * Array of the labels of each folder or file entry. This is the folder or
   * file name.
   */
  readonly labels: string[] = [];

  /** The size of each file or folder.
   *
   * For folders it's the size of the whole folder, for files it's the size of
   * each file
   */
  readonly values: number[] = [];

  /** A unique id of each file or folder. This is the full path of each entry. */
  readonly ids: string[] = [];

  /** The id (=full path) of the parent entry */
  readonly parents: string[] = [];

  /** Creates a new */
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
      `${prefix}/${subdir.dirname}`,
      fsSize,
      depth + 1,
      maxDepth
    );
  });
}
