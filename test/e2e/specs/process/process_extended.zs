// Exercises the extended process surface: argv, execPath, ppid, hrtime, platform, arch.
// Distinct sentinels; 0 = all pass.
function main(): i32 {
  let args: string[] = process.argv();
  if (args.length < 1) { return 1; }        // at least argv[0]
  if (args[0].length == 0) { return 2; }

  if (process.execPath().length == 0) { return 3; }
  if (process.ppid() <= 0) { return 4; }

  let t0: i64 = process.hrtime();
  let t1: i64 = process.hrtime();
  if (t1 < t0) { return 5; }                 // monotonic non-decreasing

  if (process.platform().length == 0) { return 6; }
  if (process.arch().length == 0) { return 7; }

  return 0;
}
