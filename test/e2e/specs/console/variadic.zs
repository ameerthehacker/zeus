// Variadic (infinite-arg) console methods, TS-style: each arg is stringified and joined by a single
// space, then a newline. log()/single-arg reproduce the old behavior; log() with no args is a blank
// line. warn/error go to stderr (so "to-stderr" is absent from stdout); log/info/debug go to stdout.
console.log("a", 5, true);
console.log();
console.log("single");
console.debug("dbg", 1);
console.info("info");
console.warn("to-stderr");
console.log("after-warn");
