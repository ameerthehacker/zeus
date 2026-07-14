// Reads the ambient `global appVersion` without importing it — it is defined in the entry module.
export function readIt(): i32 {
  return appVersion;
}
