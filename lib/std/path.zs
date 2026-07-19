// @std/path — POSIX path manipulation, pure Zeus over the `string` methods.
//
//   import { join, resolve, basename, dirname, extname } from "@std/path";
//
// Zeus targets macOS + Linux, so paths are POSIX ("/"). Node's variadic `join(...parts)` and
// `resolve(...parts)` become array-taking `join(parts: string[])` — Zeus free functions are not
// variadic. `resolve` reads the working directory from the ambient `process` global.

// sep is the POSIX path segment separator ("/"). Exposed as a function since Zeus has no exported
// module-level constants.
export function sep(): string {
  return "/";
}

// delimiter is the POSIX path list delimiter (":"), as in $PATH.
export function delimiter(): string {
  return ":";
}

// joinSegs concatenates segments with a separator (module-private helper).
function joinSegs(segs: string[], separator: string): string {
  let s: string = "";
  let i: i32 = 0;
  while (i < segs.length) {
    if (i > 0) {
      s = s + separator;
    }
    s = s + segs[i];
    i = i + 1;
  }
  return s;
}

// normalize collapses "." and ".." segments and duplicate slashes, preserving a leading "/"
// (absolute) and a single trailing "/". An empty path normalizes to ".".
export function normalize(p: string): string {
  if (p.length == 0) {
    return ".";
  }
  let isAbs: boolean = p.startsWith("/");
  let hasTrailing: boolean = p.length > 1 && p.endsWith("/");

  let segs: string[] = p.split("/");
  let out: string[] = new string[];
  let i: i32 = 0;
  while (i < segs.length) {
    let seg: string = segs[i];
    i = i + 1;
    if (seg.equals("") || seg.equals(".")) {
      continue;
    }
    if (seg.equals("..")) {
      if (out.length > 0 && !out[out.length - 1].equals("..")) {
        out.pop();
      } else if (!isAbs) {
        out.push(seg);
      }
      continue;
    }
    out.push(seg);
  }

  let joined: string = joinSegs(out, "/");
  let result: string = joined;
  if (isAbs) {
    result = "/" + joined;
  } else if (result.equals("")) {
    result = ".";
  }
  if (hasTrailing && !result.endsWith("/")) {
    result = result + "/";
  }
  return result;
}

// isAbsolute reports whether a path begins at the filesystem root.
export function isAbsolute(p: string): boolean {
  return p.startsWith("/");
}

// join concatenates path segments with "/" (skipping empties) and normalizes the result.
export function join(parts: string[]): string {
  let s: string = "";
  let i: i32 = 0;
  while (i < parts.length) {
    let part: string = parts[i];
    i = i + 1;
    if (part.length == 0) {
      continue;
    }
    if (s.length == 0) {
      s = part;
    } else {
      s = s + "/" + part;
    }
  }
  if (s.length == 0) {
    return ".";
  }
  return normalize(s);
}

// absPath resolves a single path against the current working directory (module-private helper).
function absPath(p: string): string {
  if (p.startsWith("/")) {
    return normalize(p);
  }
  return normalize(process.cwd() + "/" + p);
}

// resolve builds an absolute path from left to right: an absolute segment resets the accumulator,
// otherwise segments append. The base is the current working directory.
export function resolve(parts: string[]): string {
  let acc: string = process.cwd();
  let i: i32 = 0;
  while (i < parts.length) {
    let part: string = parts[i];
    i = i + 1;
    if (part.length == 0) {
      continue;
    }
    if (part.startsWith("/")) {
      acc = part;
    } else {
      acc = acc + "/" + part;
    }
  }
  return normalize(acc);
}

// dirname returns the directory portion of a path (everything before the last segment).
export function dirname(p: string): string {
  let end: i32 = p.length;
  while (end > 1 && p.charAt(end - 1).equals("/")) {
    end = end - 1;
  }
  let trimmed: string = p.slice(0, end);
  let idx: i32 = trimmed.lastIndexOf("/");
  if (idx < 0) {
    return ".";
  }
  if (idx == 0) {
    return "/";
  }
  return trimmed.slice(0, idx);
}

// basename returns the last segment of a path (trailing slashes ignored).
export function basename(p: string): string {
  let end: i32 = p.length;
  while (end > 1 && p.charAt(end - 1).equals("/")) {
    end = end - 1;
  }
  let trimmed: string = p.slice(0, end);
  let idx: i32 = trimmed.lastIndexOf("/");
  if (idx < 0) {
    return trimmed;
  }
  return trimmed.slice(idx + 1, trimmed.length);
}

// extname returns the extension (including the leading "."), or "" if there is none. A leading dot
// on the base name (".gitignore") is not an extension, matching Node.
export function extname(p: string): string {
  let base: string = basename(p);
  let dot: i32 = base.lastIndexOf(".");
  if (dot <= 0) {
    return "";
  }
  return base.slice(dot, base.length);
}

// relative computes the relative path from `fromPath` to `toPath`, both resolved to absolute first.
export function relative(fromPath: string, toPath: string): string {
  let f: string[] = absPath(fromPath).split("/");
  let t: string[] = absPath(toPath).split("/");

  let i: i32 = 0;
  while (i < f.length && i < t.length && f[i].equals(t[i])) {
    i = i + 1;
  }

  let out: string[] = new string[];
  let j: i32 = i;
  while (j < f.length) {
    if (f[j].length > 0) {
      out.push("..");
    }
    j = j + 1;
  }
  let k: i32 = i;
  while (k < t.length) {
    if (t[k].length > 0) {
      out.push(t[k]);
    }
    k = k + 1;
  }
  return joinSegs(out, "/");
}

// ParsedPath is the structured form of a path (a subset of Node's path.ParsedPath).
export class ParsedPath {
  public root: string;
  public dir: string;
  public base: string;
  public ext: string;
  public name: string;

  public constructor(root: string, dir: string, base: string, ext: string, name: string) {
    this.root = root;
    this.dir = dir;
    this.base = base;
    this.ext = ext;
    this.name = name;
  }
}

// parse splits a path into { root, dir, base, ext, name }.
export function parse(p: string): ParsedPath {
  let dir: string = dirname(p);
  let base: string = basename(p);
  let ext: string = extname(p);
  let name: string = base.slice(0, base.length - ext.length);
  let root: string = "";
  if (p.startsWith("/")) {
    root = "/";
  }
  return new ParsedPath(root, dir, base, ext, name);
}

// format reassembles a path from a ParsedPath, mirroring `parse`. `dir` wins over `root`, and
// `base` wins over `name`+`ext`.
export function format(pp: ParsedPath): string {
  let d: string = pp.dir;
  if (d.length == 0) {
    d = pp.root;
  }
  let b: string = pp.base;
  if (b.length == 0) {
    b = pp.name + pp.ext;
  }
  if (d.length == 0) {
    return b;
  }
  if (d.endsWith("/")) {
    return d + b;
  }
  return d + "/" + b;
}
