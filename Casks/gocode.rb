cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.17.0"
  sha256 arm: "c141fe9663073ca61710e3b8854b861ab7c8d496bda16e19a28ae0a47b6faa0b",
         intel: "9e1d0cf3420a159a000e86d9bdef13354b31a91340d6951b78b856ee13c02432"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
