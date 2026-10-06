cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.8.1"
  sha256 arm: "13c25004a6df332a091ed4835aeb7d229fe40c3af887e11d84c2465543d01b00",
         intel: "e7e18d0c058c19971bd6772dbdb3b1616d7ecc5525faa24ad10cc347dd940bda"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
