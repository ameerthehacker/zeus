class NetErr extends Error {
  constructor() {
    super("NetErr", "network down");
  }
}

// A plain Error skips the NetErr clause and is caught by the later Error clause.
function main(): i32 {
  try {
    throw new Error("Error", "generic");
  } catch (e: NetErr) {
    return 1;
  } catch (e: Error) {
    return 0;
  }
  return 2;
}
