// @link declares a native library to link (-lsqlite3). Without it, sqlite3_libversion fails to link.
@link("sqlite3");

@extern("C", "sqlite3_libversion") function sqlite3_libversion(): cstr;

function main(): i32 {
  let version: string = cStrToString(sqlite3_libversion());
  if (version.length > 0) {
    return 0;
  }
  return 1;
}
