cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.5.1"
  sha256 arm: "a5b86c49f99d35bf2a0da3e1233a97caaeb4b8af034bdedeae6e4feb1ede3e3d",
         intel: "90f1cd91b2403dddcf0c862b7c545160bf7dca471408cffcf7d159069957ed05"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
