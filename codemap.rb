class Codemap < Formula
  desc "Generate a brain map of your codebase for LLM context"
  homepage "https://github.com/JordanCoin/codemap"
  url "https://github.com/JordanCoin/codemap/archive/refs/tags/v2.2.tar.gz"
  sha256 "cce594b9bba5b6edf5f2aac4c3db15b77e134365c0bedbf6d7f7c6f21e1e2404"
  license "MIT"

  depends_on "go" => :build

  resource "tree-sitter-go" do
    url "https://github.com/tree-sitter/tree-sitter-go/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-python" do
    url "https://github.com/tree-sitter/tree-sitter-python/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-javascript" do
    url "https://github.com/tree-sitter/tree-sitter-javascript/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-typescript" do
    url "https://github.com/tree-sitter/tree-sitter-typescript/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-rust" do
    url "https://github.com/tree-sitter/tree-sitter-rust/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-ruby" do
    url "https://github.com/tree-sitter/tree-sitter-ruby/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-c" do
    url "https://github.com/tree-sitter/tree-sitter-c/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-cpp" do
    url "https://github.com/tree-sitter/tree-sitter-cpp/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-java" do
    url "https://github.com/tree-sitter/tree-sitter-java/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-swift" do
    url "https://github.com/tree-sitter/tree-sitter-swift/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-bash" do
    url "https://github.com/tree-sitter/tree-sitter-bash/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-kotlin" do
    url "https://github.com/fwcd/tree-sitter-kotlin/archive/refs/heads/main.tar.gz"
  end

  resource "tree-sitter-c-sharp" do
    url "https://github.com/tree-sitter/tree-sitter-c-sharp/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-php" do
    url "https://github.com/tree-sitter/tree-sitter-php/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-dart" do
    url "https://github.com/UserNobody14/tree-sitter-dart/archive/refs/heads/master.tar.gz"
  end

  resource "tree-sitter-r" do
    url "https://github.com/r-lib/tree-sitter-r/archive/refs/heads/main.tar.gz"
  end

  def install
    # Build the main Go binary
    system "go", "build", "-o", libexec/"codemap", "."

    # Create grammars directory
    (libexec/"grammars").mkpath

    # Build and install each grammar resource
    resources.each do |r|
      r.stage do
        lang = r.name.sub("tree-sitter-", "").tr("-", "_")

        # Handle special source directories
        src_subdir = "src"
        src_subdir = "typescript/src" if lang == "typescript"
        src_subdir = "php/src" if lang == "php"

        src_dir = Pathname.pwd/src_subdir

        # Determine library extension and flags
        lib_ext = OS.mac? ? "dylib" : "so"
        cflags = OS.mac? ? %w[-dynamiclib -fPIC] : %w[-shared -fPIC]

        output_lib = libexec/"grammars/libtree-sitter-#{lang}.#{lib_ext}"

        # Prepare sources
        sources = [src_dir/"parser.c"]

        if (src_dir/"scanner.c").exist?
          sources << (src_dir/"scanner.c")
        elsif (src_dir/"scanner.cc").exist?
          # Compile C++ scanner first
          system ENV.cxx, "-c", "-fPIC", src_dir/"scanner.cc", "-o", "scanner.o", "-I#{src_dir}"
          sources << "scanner.o"
        end

        # Compile and link
        system ENV.cc, *cflags, "-o", output_lib, *sources, "-I#{src_dir}"
      end
    end

    # Create wrapper script
    (bin/"codemap").write <<~EOS
      #!/bin/bash
      export CODEMAP_GRAMMAR_DIR="#{libexec}/grammars"
      export CODEMAP_QUERY_DIR="#{libexec}/queries"
      exec "#{libexec}/codemap" "$@"
    EOS
  end

  test do
    system bin/"codemap", "."
  end
end
