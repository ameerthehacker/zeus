// process primordial — the Node-style ambient global `process` object. Like Console, it is a
// primordial class whose methods forward to zeus_process_* runtime functions (which call libc).
// The singleton is constructed in globals.zs, so `process` is available in every module without an
// import — matching Node, where `process` is a global (unlike fs/os, which are imported modules).
class Process {
    @extern("zeus_process_cwd") public    cwd(): string;
    @extern("zeus_process_getenv") public getEnv(name: string): string;
    @extern("zeus_process_hasenv") public hasEnv(name: string): boolean;
    @extern("zeus_process_setenv") public setEnv(name: string, value: string): void;
    @extern("zeus_process_chdir") public  chdir(path: string): boolean;
    @extern("zeus_process_pid") public    pid(): i32;
    @extern("zeus_process_ppid") public   ppid(): i32;
    @extern("zeus_process_exit") public   exit(code: i32): void;
    @extern("zeus_process_argv") public   argv(): string[];
    @extern("zeus_process_exec_path") public execPath(): string;
    @extern("zeus_process_hrtime") public hrtime(): i64;
    @extern("zeus_process_platform") public platform(): string;
    @extern("zeus_process_arch") public   arch(): string;
}
