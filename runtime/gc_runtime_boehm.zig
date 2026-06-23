const c = @cImport({
    @cInclude("gc.h");
});

pub export fn zeus_gc_alloc(size: u32) ?*anyopaque {
    return c.GC_malloc(size);
}
