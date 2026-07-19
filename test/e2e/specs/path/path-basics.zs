import { join, resolve, normalize, dirname, basename, extname, isAbsolute, relative, parse, format, ParsedPath, sep } from "@std/path";


function main(): i32 {
  console.log(join(["usr", "local", "bin"]));       // usr/local/bin
  console.log(join(["/a", "b", "..", "c"]));         // /a/c
  console.log(normalize("/a/./b/../c//d"));          // /a/c/d
  console.log(dirname("/usr/local/bin"));            // /usr/local
  console.log(basename("/usr/local/bin/zeus"));      // zeus
  console.log(extname("archive.tar.gz"));            // .gz
  console.log(extname("README"));                    // (empty)
  console.log(sep());                                // /

  if (!isAbsolute("/etc")) { return 1; }
  if (isAbsolute("etc")) { return 2; }
  if (!join(["a", "b"]).equals("a/b")) { return 3; }
  if (!normalize("a/b/../c").equals("a/c")) { return 4; }
  if (!dirname("a/b/c").equals("a/b")) { return 5; }
  if (!basename("a/b/c.txt").equals("c.txt")) { return 6; }
  if (!extname("c.txt").equals(".txt")) { return 7; }
  if (!extname(".gitignore").equals("")) { return 8; }
  if (!relative("/a/b/c", "/a/b/d/e").equals("../d/e")) { return 9; }

  let pp: ParsedPath = parse("/usr/local/zeus.tar.gz");
  if (!pp.dir.equals("/usr/local")) { return 10; }
  if (!pp.base.equals("zeus.tar.gz")) { return 11; }
  if (!pp.ext.equals(".gz")) { return 12; }
  if (!pp.name.equals("zeus.tar")) { return 13; }
  if (!format(pp).equals("/usr/local/zeus.tar.gz")) { return 14; }

  console.log("path-ok");
  return 0;
}
