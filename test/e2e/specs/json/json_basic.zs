// JSON and JsonValue are global (no import), like console/Math. Distinct sentinels; 0 = all pass.
function main(): i32 {
  let v: JsonValue = JSON.parse("{\"name\":\"zeus\",\"stars\":[1,2,3],\"ok\":true,\"nil\":null}");
  if (!v.get("name").asString().equals("zeus")) { return 1; }
  if (v.get("stars").length() != 3) { return 2; }
  if (v.get("stars").at(2).asInt() != 3) { return 3; }
  if (!v.get("ok").asBool()) { return 4; }
  if (!v.get("nil").isNull()) { return 5; }
  if (v.has("missing")) { return 6; }

  let obj: JsonValue = JSON.newObject();
  obj.set("id", JSON.newNumber(42.0));
  obj.set("tag", JSON.newString("hi"));
  let arr: JsonValue = JSON.newArray();
  arr.push(JSON.newBool(true));
  arr.push(JSON.newNull());
  obj.set("list", arr);
  if (!JSON.stringify(obj).equals("{\"id\":42,\"tag\":\"hi\",\"list\":[true,null]}")) { return 7; }

  // escaping round-trip
  let s: string = "{\"msg\":\"a\\\"b\\nc\"}";
  if (!JSON.stringify(JSON.parse(s)).equals(s)) { return 8; }

  // nested access
  let n: JsonValue = JSON.parse("[{\"a\":[10,20]},{\"a\":[30]}]");
  if (n.at(0).get("a").at(1).asInt() != 20) { return 9; }
  if (n.at(1).get("a").at(0).asInt() != 30) { return 10; }

  // objectKeys
  let keys: string[] = v.objectKeys();
  if (keys.length != 4) { return 11; }

  return 0;
}
