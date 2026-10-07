cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.15.0"
  sha256 arm: "efc14f2c8011faaba087630968714af6f5f5c0501cb4cdc0df6fe7d1dd8bf1a9",
         intel: "07a9984e470566098d8d8fc59cfc7a89dc5f05ecc48a58043f73daae3a537f95"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
