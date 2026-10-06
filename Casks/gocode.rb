cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.5.0"
  sha256 arm: "29366feb8cc4edc4e510e98dea61730dddafafb8b3cfe479146011379d24e0e7",
         intel: "f4f74bf3857866be2015ae52d51cae34c0a690415b554898243401a005490c16"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
