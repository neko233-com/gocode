cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.21.0"
  sha256 arm: "19e8c0ca60b28a5debace09342be0fb3f47e589bc6ae0408215fabf7453e4c86",
         intel: "f88de055853974b6aaf4938627099e3d6a41db7edcf87c6919dce347996c4f48"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
