; ModuleID = '/Users/ameerthehacker/Projects/zeus/playground/hello.tsl'
source_filename = "/Users/ameerthehacker/Projects/zeus/playground/hello.tsl"

define i64 @main() {
"0":
  %a = alloca i16, align 2
  store i16 4, ptr %a, align 2
  %b = alloca i64, align 8
  store i64 2, ptr %b, align 4
  %a1 = load i16, ptr %a, align 2
  %b2 = load i64, ptr %b, align 4
  %i64_cast = sext i16 %a1 to i64
  %div = sdiv i64 %i64_cast, %b2
  %add = add i64 %div, 1
  %c = alloca i64, align 8
  store i64 %add, ptr %c, align 4
  %c3 = load i64, ptr %c, align 4
  ret i64 %c3
}
