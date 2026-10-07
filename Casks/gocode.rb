cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.20.0"
  sha256 arm: "934ff8d4b0002b9e703a3d22426627200fd0590eee33a75121b8573e4eace254",
         intel: "68d9daa2b45589fecdd9d8167cc3bfc5895f3d301d69c59909daf18b3b0aef88"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
