// process primordial — the Node-style ambient global `process` object. Like Console, it is a
// primordial class whose methods forward to zeus_process_* runtime functions (which call libc).
// The singleton is constructed in globals.zs, so `process` is available in every module without an
// import — matching Node, where `process` is a global (unlike fs/os, which are imported modules).
class Process {
    public extern("zeus_process_cwd")    cwd(): string;
    public extern("zeus_process_getenv") getEnv(name: string): string;
    public extern("zeus_process_hasenv") hasEnv(name: string): boolean;
    public extern("zeus_process_setenv") setEnv(name: string, value: string): void;
    public extern("zeus_process_chdir")  chdir(path: string): boolean;
    public extern("zeus_process_pid")    pid(): i32;
    public extern("zeus_process_exit")   exit(code: i32): void;
}
