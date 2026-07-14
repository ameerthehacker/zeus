// Ambient globals available in every module without import. `Console` and `Math` are primordial
// classes (see console.zs / math.zs); here we construct their program-wide singleton instances.
// This module is always compiled and linked, and its init runs first at startup, so `console` and
// `Math` are ready before any user code.
global console = new Console();
global Math = new Math();
