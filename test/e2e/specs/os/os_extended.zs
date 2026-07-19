import { freemem, totalmem, availableParallelism, type, release, version, machine, endianness, EOL, loadavg, LoadAvg } from "@std/os";

// Exercises the extended @std/os surface. Distinct sentinels; 0 = all pass.
function main(): i32 {
  if (type().length == 0) { return 1; }
  if (release().length == 0) { return 2; }
  if (version().length == 0) { return 3; }
  if (machine().length == 0) { return 4; }
  if (totalmem() <= 0) { return 5; }
  if (freemem() < 0) { return 6; }
  if (availableParallelism() < 1) { return 7; }
  if (!endianness().equals("LE")) { return 8; }
  if (!EOL().equals("\n")) { return 9; }

  let la: LoadAvg = loadavg();
  if (la.one < 0.0) { return 10; }
  if (la.five < 0.0) { return 11; }
  if (la.fifteen < 0.0) { return 12; }

  return 0;
}
