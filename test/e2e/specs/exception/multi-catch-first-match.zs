class NetErr extends Error {
  constructor() {
    super("NetErr", "network down");
  }
}

function main(): i32 {
  try {
    throw new NetErr();
  } catch (e: NetErr) {
    return 0;
  } catch (e: Error) {
    return 1;
  }
  return 2;
}
