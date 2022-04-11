import { expect } from "chai";
import { it, describe } from "mocha";

import { compareDirsToDataNodes, type Dir } from "../src/fs-tree";

describe("compareDirsToDataNodes", () => {
  const d1: Dir = {
    dirname: "/",
    total_size: 100,
    files: { secret: 10, foobar: 20 },
    directories: {
      etc: {
        total_size: 10,
        dirname: "etc",
        directories: {},
        files: { "os-release": 10 }
      },
      tmp: {
        total_size: 0,
        dirname: "tmp",
        directories: {},
        files: {}
      }
    }
  };

  const d2: Dir = {
    dirname: "/",
    total_size: 101,
    files: { asdf: 15, foobar: 15, iAmOnlyInD2: 3 },
    directories: {
      etc: {
        total_size: 10,
        dirname: "etc",
        directories: {},
        files: { "os-release": 10, product: 1 }
      }
    }
  };

  it("creates the correct data structures", () => {
    const [node1, node2] = compareDirsToDataNodes(d1, d2);
    expect(node1).to.deep.equal({
      name: "/",
      value: 0,
      children: [
        { name: "secret", value: 10, color: "blue" },
        { name: "foobar", color: "red", value: 20 },
        {
          name: "etc",
          color: "yellow",
          value: 0,
          children: [{ name: "os-release", value: 10, color: "yellow" }]
        },
        { name: "tmp", value: 0, color: "blue", children: [] }
      ],
      color: "green"
    });
    expect(node2).to.deep.equal({
      name: "/",
      value: 0,
      children: [
        { name: "asdf", value: 15, color: "blue" },
        { name: "foobar", value: 15, color: "green" },
        { name: "iAmOnlyInD2", color: "blue", value: 3 },
        {
          name: "etc",
          color: "yellow",
          value: 0,
          children: [
            { name: "os-release", value: 10, color: "yellow" },
            { name: "product", value: 1, color: "blue" }
          ]
        }
      ],
      color: "red"
    });
  });
});
