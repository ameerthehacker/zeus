// @std/buffer — a Buffer type over Zeus's native u8[], plus little-endian integer accessors and
// hex/base64/utf8 conversions. A subset of Node's Buffer.
//
//   import { Buffer, alloc, fromString } from "@std/buffer";
//   let b: Buffer = fromString("hello");
//   console.log(b.toHex());
//
// Zeus cannot resolve a static method across modules yet, so Node's Buffer.alloc/Buffer.from are
// exposed as the free functions alloc / fromString / fromHex / fromBase64. `new Buffer(bytes)` wraps
// an existing u8[] directly.

import { hexEncode, hexDecode, base64Encode, base64Decode } from "@std/encoding";

export class Buffer {
  private data: u8[];

  public constructor(data: u8[]) {
    this.data = data;
  }

  public length(): i32 {
    return this.data.length;
  }

  public get(index: i32): u8 {
    return this.data[index];
  }

  public set(index: i32, value: u8): void {
    this.data[index] = value;
  }

  // bytes returns the underlying u8[] (not a copy).
  public bytes(): u8[] {
    return this.data;
  }

  // toString decodes the buffer as UTF-8 text.
  public toString(): string {
    return this.data;
  }

  public toHex(): string {
    return hexEncode(this.data);
  }

  public toBase64(): string {
    return base64Encode(this.data);
  }

  public equals(other: Buffer): boolean {
    if (this.data.length != other.length()) {
      return false;
    }
    let i: i32 = 0;
    while (i < this.data.length) {
      if (this.data[i] != other.get(i)) {
        return false;
      }
      i = i + 1;
    }
    return true;
  }

  // slice returns a copy of bytes in [start, end).
  public slice(start: i32, end: i32): Buffer {
    let out: u8[] = new u8[];
    let i: i32 = start;
    while (i < end && i < this.data.length) {
      out.push(this.data[i]);
      i = i + 1;
    }
    return new Buffer(out);
  }

  public readUInt8(offset: i32): i32 {
    return (this.data[offset] as i32) & 255;
  }

  public writeUInt8(value: i32, offset: i32): void {
    this.data[offset] = (value & 255) as u8;
  }

  public readUInt16LE(offset: i32): i32 {
    return ((this.data[offset] as i32) & 255) | (((this.data[offset + 1] as i32) & 255) << 8);
  }

  public writeUInt16LE(value: i32, offset: i32): void {
    this.data[offset] = (value & 255) as u8;
    this.data[offset + 1] = ((value >> 8) & 255) as u8;
  }

  public readUInt32LE(offset: i32): i64 {
    let b0: i64 = (this.data[offset] as i64) & 255;
    let b1: i64 = (this.data[offset + 1] as i64) & 255;
    let b2: i64 = (this.data[offset + 2] as i64) & 255;
    let b3: i64 = (this.data[offset + 3] as i64) & 255;
    return b0 | (b1 << 8) | (b2 << 16) | (b3 << 24);
  }

  public writeUInt32LE(value: i64, offset: i32): void {
    this.data[offset] = (value & 255) as u8;
    this.data[offset + 1] = ((value >> 8) & 255) as u8;
    this.data[offset + 2] = ((value >> 16) & 255) as u8;
    this.data[offset + 3] = ((value >> 24) & 255) as u8;
  }
}

// alloc returns a Buffer of `size` zero bytes.
export function alloc(size: i32): Buffer {
  let data: u8[] = new u8[];
  let i: i32 = 0;
  while (i < size) {
    data.push(0 as u8);
    i = i + 1;
  }
  return new Buffer(data);
}

// fromString builds a Buffer from a string's UTF-8 bytes.
export function fromString(s: string): Buffer {
  let data: u8[] = s;
  return new Buffer(data);
}

// fromHex builds a Buffer by decoding a hex string.
export function fromHex(s: string): Buffer {
  return new Buffer(hexDecode(s));
}

// fromBase64 builds a Buffer by decoding a base64 string.
export function fromBase64(s: string): Buffer {
  return new Buffer(base64Decode(s));
}

// concat joins two buffers into a new one.
export function concat(a: Buffer, b: Buffer): Buffer {
  let out: u8[] = new u8[];
  let i: i32 = 0;
  while (i < a.length()) {
    out.push(a.get(i));
    i = i + 1;
  }
  let j: i32 = 0;
  while (j < b.length()) {
    out.push(b.get(j));
    j = j + 1;
  }
  return new Buffer(out);
}
