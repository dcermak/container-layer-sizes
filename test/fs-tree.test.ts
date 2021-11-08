import { Dir, FsSize } from "../src/fs-tree";
import { describe, it } from "mocha";
import { expect } from "chai";

describe("FsSize", () => {
  it("is empty when Dir is empty", () => {
    const fS = new FsSize({
      path: "/",
      total_size: 0,
      files: {},
      directories: {}
    });

    expect(fS.labels.length).to.equal(0);
  });

  it("Correctly creates the fs structure", () => {
    const d: Dir = {
      path: "/",
      total_size: 1000,
      files: {
        file_1: 1,
        file_2: 2
      },
      directories: {
        etc: {
          total_size: 100,
          path: "etc",
          files: { "os-release": 3, passwd: 4 },
          directories: {}
        }
      }
    };

    const fS = new FsSize(d);

    expect(fS.labels).to.deep.equal([
      "file_1",
      "file_2",
      "etc",
      "os-release",
      "passwd"
    ]);
    expect(fS.ids).to.deep.equal([
      "/file_1",
      "/file_2",
      "/etc",
      "/etc/os-release",
      "/etc/passwd"
    ]);
    expect(fS.values).to.deep.equal([1, 2, 100, 3, 4]);
    expect(fS.parents).to.deep.equal(["", "", "", "/etc", "/etc"]);
  });
});
