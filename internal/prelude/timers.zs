// Timer primordials — runtime-backed free functions. Self-hosted prelude; each forwards to its
// zeus_* runtime symbol and is registered as a primordial function when the prelude loads.
extern("zeus_setTimeout") function setTimeout(callback: () => void, delay: i32): i32;
extern("zeus_clearTimeout") function clearTimeout(id: i32): void;
extern("zeus_setInterval") function setInterval(callback: () => void, delay: i32): i32;
extern("zeus_clearInterval") function clearInterval(id: i32): void;
