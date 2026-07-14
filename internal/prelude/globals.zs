// Ambient globals available in every module without import. `Console` is a primordial class (see
// console.zs); here we construct its program-wide singleton instance. This module is always
// compiled and linked, and its init runs first at startup, so `console` is ready before any user
// code. (`Math` is a pure static class — see math.zs — so it needs no singleton here.)
global console = new Console();
