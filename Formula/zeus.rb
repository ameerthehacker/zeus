# Zeus Programming Language Homebrew Formula
#
# This formula is automatically updated by the release workflow.
#
# Installation:
#   brew tap ameerthehacker/zeus https://github.com/ameerthehacker/zeus
#   brew install zeus

class Zeus < Formula
  desc "Zeus programming language compiler"
  homepage "https://github.com/ameerthehacker/zeus"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/ameerthehacker/zeus/releases/download/v#{version}/zeus-#{version}-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER_ARM64_SHA256"
    end

    on_intel do
      url "https://github.com/ameerthehacker/zeus/releases/download/v#{version}/zeus-#{version}-darwin-x86_64.tar.gz"
      sha256 "PLACEHOLDER_X86_64_SHA256"
    end
  end

  def install
    # Install the binary
    bin.install "bin/zeus"

    # Install runtime (zig-out directory with compiled runtime)
    (prefix/"runtime/zig-out").install Dir["runtime/zig-out/*"]

    # Install standard library
    (prefix/"lib").install Dir["lib/*"]
  end

  def caveats
    <<~EOS
      Zeus has been installed successfully!

      The Zeus compiler can now find its runtime and standard library automatically.
      No additional configuration is needed.

      To compile a Zeus program:
        zeus build your_program.zs -o your_program

      To run it:
        ./your_program
    EOS
  end

  test do
    # Create a simple test program
    (testpath/"hello.zs").write <<~EOS
      fn main() {
        print("Hello, World!")
      }
    EOS

    # Test that the compiler can compile a simple program
    system "#{bin}/zeus", "build", "hello.zs", "-o", "hello"
  end
end

